package logger

import (
	"bytes"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

type remoteLogger struct {
	logBigFileChan chan *LogFile
	parent         *Logger
	quitChan       chan byte
	pasteURL       *url.URL
	httpClient     *retryablehttp.Client
}

func newRemoteLogger(parent *Logger) *remoteLogger {
	defaultPasteURL, _ := url.Parse("https://paste.macports.org/")
	pasteURL := defaultPasteURL
	if envPasteURL := os.Getenv("PASTE_URL"); envPasteURL != "" {
		var err error
		pasteURL, err = url.Parse(envPasteURL)
		if err != nil {
			log.Println(err)
			pasteURL = defaultPasteURL
		}
	}
	r := &remoteLogger{
		logBigFileChan: make(chan *LogFile, 4),
		parent:         parent,
		quitChan:       make(chan byte),
		pasteURL:       pasteURL,
		httpClient:     retryablehttp.NewClient(),
	}
	r.httpClient.Logger = nil
	r.httpClient.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return r
}

func (r *remoteLogger) run() {
	for {
		var err error
		var fieldName string

		select {
		case logBigFile := <-r.logBigFileChan:
			if logBigFile == nil {
				r.parent.LogChan <- &LogText{FieldName: "", Text: nil}
				return
			}
			fieldName = logBigFile.FieldName
			fileInfo, iErr := os.Stat(logBigFile.Filename)
			if iErr != nil {
				err = iErr
				break
			}
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
			postForm := bytes.NewReader(buf.Bytes())
			resp, iErr := r.httpClient.Post(r.pasteURL.String(), mimeWriter.FormDataContentType(), postForm)
			if iErr != nil {
				err = iErr
				break
			}
			resp.Body.Close()
			loc := resp.Header.Get("Location")
			if loc == "" {
				err = errors.New("missing Location header")
				break
			}
			u, iErr := r.pasteURL.Parse(loc)
			if iErr != nil {
				err = iErr
				break
			}
			r.parent.LogChan <- &LogText{
				fieldName + "-pastebin",
				[]byte(u.String()),
			}
		}

		if err != nil {
			r.parent.LogChan <- &LogText{
				fieldName + "-pastebin-fail",
				[]byte(err.Error()),
			}
		}
	}
}
