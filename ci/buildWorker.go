package ci

import (
	"os/exec"
	"path"

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
				logger.GlobalLogger.LogTextChan <- &logger.LogText{"port-" + port + "-subports-fail", []byte(err.Error())}
				continue
			}
			for _, subport := range subports {
				statusString := "success"
				DeactivateAllPorts()
				portTmpDir := path.Join(worker.session.tmpDir, subport)
				logFilename := path.Join(worker.session.tmpDir, "port-"+subport+"-dep-install.log")
				err := mpbbToLog("install-dependencies", subport, portTmpDir, logFilename)
				if err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						if !eerr.Success() {
							returnCode = 1
							statusString = "fail"
						}
					}
				}
				logger.GlobalLogger.LogFileChan <- &logger.LogFile{
					"port-" + subport + "-dep-summary-" + statusString,
					path.Join(portTmpDir, "logs/dependencies-progress.txt"),
				}
				if err != nil {
					logger.GlobalLogger.LogBigFileChan <- &logger.LogFile{
						"port-" + port + "-dep-install-output-" + statusString,
						logFilename,
					}
					continue
				}

				logFilename = path.Join(worker.session.tmpDir, "port-"+subport+"-install.log")
				err = mpbbToLog("install-port", subport, portTmpDir, logFilename)
				if err != nil {
					if eerr, ok := err.(*exec.ExitError); ok {
						if !eerr.Success() {
							returnCode = 1
							statusString = "fail"
						}
					}
				}
				logger.GlobalLogger.LogFileChan <- &logger.LogFile{
					"port-" + subport + "-install-summary-" + statusString,
					path.Join(portTmpDir, "logs/ports-progress.txt"),
				}
				logger.GlobalLogger.LogBigFileChan <- &logger.LogFile{
					"port-" + port + "-install-output-" + statusString,
					logFilename,
				}
			}
		}
	}
}
