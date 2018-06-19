package ci

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/macports/mpbot-github/ci/logger"
)

// buildWorker receive ports from portChan
type buildWorker struct {
	worker
	portChan chan string
}

func newBuildWorker(session *Session) *buildWorker {
	return &buildWorker{
		worker:   worker{session: session, quitChan: make(chan byte)},
		portChan: make(chan string),
	}
}

func (worker *buildWorker) start() {
	returnCode := byte(0)
	for {
		select {
		case port := <-worker.portChan:
			if port == "" {
				worker.quitChan <- returnCode
				return
			}
			subports, err := ListSubports(port)
			if err != nil {
				returnCode = 1
				logger.GlobalLogger.LogChan <- &logger.LogText{"port-" + port + "-subports-fail", []byte(err.Error())}
				continue
			}
			logger.GlobalLogger.LogChan <- &logger.LogText{"port-" + port + "-subports", []byte(strings.Join(subports, "\n"))}
			for _, subport := range subports {
				statusString := "success"
				DeactivateAllPorts()
				portTmpDir := path.Join(worker.session.tmpDir, subport)
				logFilename := path.Join(worker.session.tmpDir, "port-"+subport+"-dep-install.log")
				logger.GlobalLogger.LogChan <- &logger.LogText{"port-" + subport + "-dep-install-start", nil}
				err := mpbbToLog("install-dependencies", subport, portTmpDir, logFilename)
				if err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						if !eerr.Success() {
							returnCode = 1
							statusString = "fail"
						}
					}
				}
				logger.GlobalLogger.LogChan <- &logger.LogFile{
					FieldName: "port-" + subport + "-dep-summary-" + statusString,
					Filename:  path.Join(portTmpDir, "logs/dependencies-progress.txt"),
				}
				if err != nil {
					logger.GlobalLogger.LogChan <- &logger.LogFile{
						FieldName: "port-" + subport + "-dep-install-output-" + statusString,
						Filename:  logFilename,
						Big:       true,
					}
					continue
				}

				logFilename = path.Join(worker.session.tmpDir, "port-"+subport+"-install.log")
				logger.GlobalLogger.LogChan <- &logger.LogText{"port-" + subport + "-install-start", nil}
				err = mpbbToLog("install-port", subport, portTmpDir, logFilename)
				if err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						if !eerr.Success() {
							returnCode = 1
							statusString = "fail"
						}
					}
				}

				// DEBUG START
				logFileInfo, err := os.Stat(logFilename)
				if err != nil {
					log.Println(err)
					continue
				}
				file, err := os.Open(logFilename)
				if err != nil {
					log.Println(err)
					continue
				}
				// Max 4 KiB
				if fileSize := logFileInfo.Size(); fileSize > 4*1024 {
					file.Seek(fileSize-4*1024, 0)
				}
				logTail, err := ioutil.ReadAll(file)
				file.Close()
				logger.GlobalLogger.LogChan <- &logger.LogText{
					FieldName: "port-" + subport + "-install-output-tail",
					Text:      logTail,
				}
				// DEBUG END

				logger.GlobalLogger.LogChan <- &logger.LogFile{
					FieldName: "port-" + subport + "-install-summary-" + statusString,
					Filename:  path.Join(portTmpDir, "logs/ports-progress.txt"),
				}
				logger.GlobalLogger.LogChan <- &logger.LogFile{
					FieldName: "port-" + subport + "-install-output-" + statusString,
					Filename:  logFilename,
					Big:       true,
				}
			}
		}
	}
}
