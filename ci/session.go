package ci

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/macports/mpbot-github/ci/logger"
)

// Session is the public interface of the ci package.
// workers are not exported.
type Session struct {
	tmpDir       string
	ports        []string
	buildCounter uint32
	// TODO: split to lintResults and buildResults or use unified struct type
	// TODO: Return error in Run() if build failed
	// Collects lint and build failure
	results chan string
}

func NewSession() (*Session, error) {
	tmpDir, err := ioutil.TempDir("", "ci-build-")
	if err != nil {
		return nil, err
	}
	ports, err := GetChangedPortList()
	if err != nil {
		// No cleanup in CI
		//os.RemoveAll(tmpDir)
		return nil, err
	}
	return &Session{
		tmpDir:  tmpDir,
		ports:   ports,
		results: make(chan string),
	}, nil
}

// Run() blocks until all ports are tested and all logs are printed
// or queued in the GlobalLogger.
func (session *Session) Run() error {
	logger.GlobalLogger.LogChan <- &logger.LogText{"port-list", []byte(strings.Join(session.ports, "\n"))}
	if len(session.ports) == 0 {
		return nil
	}
	var err error
	bWorker := newBuildWorker(session)
	go bWorker.start()
	go func() {
		for _, port := range session.ports {
			bWorker.portChan <- port
		}
		bWorker.portChan <- ""
	}()

	{
		lintArgs := make([]string, len(session.ports)+2)
		lintArgs[0] = "-p"
		lintArgs[1] = "lint"
		copy(lintArgs[2:], session.ports)
		lintCmd := exec.Command("port", lintArgs...)
		out, err := lintCmd.CombinedOutput()
		statusString := "success"
		if err != nil {
			statusString = "fail"
			err = errors.New("lint failed")
		}
		logger.GlobalLogger.LogChan <- &logger.LogText{"port-lint-output-" + statusString, out}
	}

	if bWorker.wait() != 0 {
		err = errors.New("build failed")
	}
	return err
}
