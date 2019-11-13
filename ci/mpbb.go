package ci

import (
	"bufio"
	"os"
	"os/exec"
)

// List all subports of a given port.
func ListSubports(port string) ([]string, error) {
	listCmd := exec.Command("mpbb", "list-subports", "--archive-site=", "--archive-site-private=", port)
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
func mpbbToLog(command, port, workDir, logFilePath string, commandArg ...string) error {
	var mpbbCmd *exec.Cmd
	args := make([]string, 0, 2)
	if workDir != "" {
		args = append(args, "--work-dir", workDir)
	}
	args = append(args, command)
	args = append(args, commandArg...)
	args = append(args, port)

	mpbbCmd = exec.Command("mpbb", args...)

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
