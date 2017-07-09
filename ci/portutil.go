package ci

import (
	"bufio"
	"os/exec"
)

// Only deactivate ports to save time when a dependency
// is needed across builds. It should be able to avoid
// conflicts.
func DeactivateAllPorts() {
	deactivateCmd := exec.Command("port", "deactivate", "active")
	deactivateCmd.Start()
	deactivateCmd.Wait()
}

// List all subports of a given port.
func ListSubports(port string) ([]string, error) {
	listCmd := exec.Command("port", "-q", "info", "--index", "--line", "--name", port, "subportof:"+port)
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
