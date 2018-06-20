package ci

import (
	"bufio"
	"os"
	"os/exec"
)

// mpbbToLog executes `mpbb` and saves output to a file at logFilePath.
func mpbbToLog(command, port, workDir, logFilePath string) error {
	var mpbbCmd *exec.Cmd
	if workDir != "" {
		mpbbCmd = exec.Command("mpbb", "--work-dir", workDir, command, port)
	} else {
		mpbbCmd = exec.Command("mpbb", command, port)
	}

	logFile, err := os.Create(logFilePath)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logWriter := bufio.NewWriter(logFile)
	defer logWriter.Flush()
	// other logs in workDir/logs
	mpbbCmd.Stdout = logWriter
	mpbbCmd.Stderr = logWriter
	if err = mpbbCmd.Run(); err != nil {
		return err
	}
	return nil
}
