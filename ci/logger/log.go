package logger

import (
	"io"
	"mime/multipart"
	"os"
	"time"
)

// This is the actual logger used by the CI bot
var GlobalLogger *Logger = newLogger(os.Stdout)

func init() {
	go GlobalLogger.Run()
}

// Logger only exits when nil is send to its LogTextChan.
type Logger struct {
	LogChan      chan interface{}
	mimeWriter   *multipart.Writer
	quitChan     chan byte
	remoteLogger *remoteLogger
}

type LogFile struct {
	FieldName, Filename string
	Big                 bool
}

type LogText struct {
	FieldName string
	Text      []byte
}

func newLogger(w io.Writer) *Logger {
	logger := &Logger{
		LogChan:    make(chan interface{}),
		mimeWriter: multipart.NewWriter(w),
		quitChan:   make(chan byte),
	}
	logger.remoteLogger = newRemoteLogger(logger)
	return logger
}

func (l *Logger) Run() {
	go l.remoteLogger.run()

	var log interface{}
	for {
		select {
		case log = <-l.LogChan:
		case <-time.After(time.Minute * 5):
			log = &LogText{"keep-alive", []byte{}}
		}

		if log == nil {
			l.remoteLogger.logBigFileChan <- nil
			continue
		}

		logText, ok := log.(*LogText)
		if ok {
			if logText.FieldName == "" {
				l.mimeWriter.Close()
				l.quitChan <- 0
				return
			}
			writer, err := l.mimeWriter.CreateFormField(logText.FieldName)
			if err != nil {
				continue
			}
			_, err = writer.Write(logText.Text)
			continue
		}

		logFile, ok := log.(*LogFile)
		if ok {
			if logFile.Big {
				l.remoteLogger.logBigFileChan <- logFile
				continue
			}
			writer, err := l.mimeWriter.CreateFormField(logFile.FieldName)
			if err != nil {
				continue
			}
			file, err := os.Open(logFile.Filename)
			if err != nil {
				continue
			}

			io.Copy(writer, file)

			file.Close()
			//os.Remove(logFile.Filename)
			continue
		}
	}
}

func (l *Logger) Wait() byte {
	return <-l.quitChan
}
