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
	LogTextChan    chan *LogText
	LogFileChan    chan *LogFile
	LogBigFileChan chan *LogFile
	mimeWriter     *multipart.Writer
	quitChan       chan byte
	remoteLogger   *remoteLogger
}

type LogFile struct {
	FieldName, Filename string
}

type LogText struct {
	FieldName string
	Text      []byte
}

func newLogger(w io.Writer) *Logger {
	logger := &Logger{
		mimeWriter:     multipart.NewWriter(w),
		quitChan:       make(chan byte),
		LogTextChan:    make(chan *LogText, 4),
		LogFileChan:    make(chan *LogFile, 4),
		LogBigFileChan: make(chan *LogFile, 4),
	}
	logger.remoteLogger = newRemoteLogger(logger)
	return logger
}

func (l *Logger) Run() {
	go l.remoteLogger.run()
	for {
		select {
		case logFile := <-l.LogFileChan:
			if logFile == nil {
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
			os.Remove(logFile.Filename)
		case logText := <-l.LogTextChan:
			if logText == nil {
				l.remoteLogger.logBigFileChan <- nil
				continue
			}
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
		case logBigFile := <-l.LogBigFileChan:
			l.remoteLogger.logBigFileChan <- logBigFile
		case <-time.After(time.Minute * 5):
			l.LogTextChan <- &LogText{"keep-alive", []byte{}}
		}
	}
}

func (l *Logger) Wait() byte {
	return <-l.quitChan
}
