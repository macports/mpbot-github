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
				r.parent.LogTextChan <- &LogText{FieldName: "", Text: nil}
				return
			}
			for i := 1; i < 3; i++ {
				fileInfo, err := os.Stat(logBigFile.Filename)
				if err != nil {
					// TODO: print err in log
					break
				}
				file, err := os.Open(logBigFile.Filename)
				if err != nil {
					break
				}
				buf := new(bytes.Buffer)
				mimeWriter := multipart.NewWriter(buf)
				writer, err := mimeWriter.CreateFormField("paste")
				if err != nil {
					break
				}
				// Max 8 MiB
				if fileSize := fileInfo.Size(); fileSize > 8*1024*1024 {
					file.Seek(fileSize-8*1024*1024, 0)
				}
				io.Copy(writer, file)
				file.Close()
				mimeWriter.Close()
				resp, err := r.httpClient.Post(pasteURL.String(), mimeWriter.FormDataContentType(), buf)
				if err != nil {
					continue
				}
				loc := resp.Header.Get("Location")
				if loc == "" {
					continue
				}
				u, err := pasteURL.Parse(loc)
				if err != nil {
					break
				}
				GlobalLogger.LogTextChan <- &LogText{logBigFile.FieldName + "-pastebin", []byte(u.String())}
				break
			}
		}
	}
}
