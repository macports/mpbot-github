package logger

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
)

var pasteURL = &url.URL{
	Scheme: "https",
	Host:   "paste.macports.org",
	Path:   "/",
}

type remoteLogger struct {
	logBigFileChan chan *LogFile
	parent         *Logger
	quitChan       chan byte
	httpClient     *http.Client
}

func newRemoteLogger(parent *Logger) *remoteLogger {
	return &remoteLogger{
		logBigFileChan: make(chan *LogFile, 4),
		parent:         parent,
		quitChan:       make(chan byte),
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (r *remoteLogger) run() {
	for {
		select {
		case logBigFile := <-r.logBigFileChan:
			if logBigFile == nil {
				r.parent.LogChan <- &LogText{FieldName: "", Text: nil}
				return
			}
			fileInfo, err := os.Stat(logBigFile.Filename)
			if err == nil {
				for i := 1; i < 3; i++ {
					file, iErr := os.Open(logBigFile.Filename)
					if iErr != nil {
						err = iErr
						break
					}
					buf := new(bytes.Buffer)
					mimeWriter := multipart.NewWriter(buf)
					writer, iErr := mimeWriter.CreateFormField("paste")
					if iErr != nil {
						err = iErr
						break
					}
					// Max 8 MiB
					const trimNote = "*log trimmed*\n"
					if fileSize := fileInfo.Size(); fileSize > 8*1024*1024-int64(len(trimNote)) {
						file.Seek(fileSize-8*1024*1024+int64(len(trimNote)), 0)
						io.WriteString(writer, trimNote)
					}
					io.Copy(writer, file)
					file.Close()
					mimeWriter.Close()
					resp, iErr := r.httpClient.Post(pasteURL.String(), mimeWriter.FormDataContentType(), buf)
					if iErr != nil {
						err = iErr
						continue
					}
					resp.Body.Close()
					loc := resp.Header.Get("Location")
					if loc == "" {
						err = iErr
						continue
					}
					u, iErr := pasteURL.Parse(loc)
					if iErr != nil {
						err = iErr
						break
					}
					r.parent.LogChan <- &LogText{
						logBigFile.FieldName + "-pastebin",
						[]byte(u.String()),
					}
					err = nil
					break
				}
			}
			if err != nil {
				r.parent.LogChan <- &LogText{
					logBigFile.FieldName + "-pastebin-fail",
					[]byte(err.Error()),
				}
			}
		}
	}
}
