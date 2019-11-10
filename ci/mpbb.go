package ci

import (
	"bufio"
	"os"
	"os/exec"
)

// List all subports of a given port.
func ListSubports(port, workDir string) ([]string, error) {
	listCmd := exec.Command("mpbb", "--work-dir", workDir, "list-subports", "--archive-site=", "--archive-site-private=", port)
	stdout, err := listCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = listCmd.Start(); err != nil {
		return nil, err
	}
	subports := make([]string, 0, 1)
	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		subports = append(subports, line)
	}
	if err = listCmd.Wait(); err != nil {
		return nil, err
	}
	return subports, nil
}

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
