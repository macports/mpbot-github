package ci

import (
	"bufio"
	"os/exec"
	"regexp"
)

// List second part of path of Portfiles (or port files) that was changed in the PR.
func GetChangedPortList() ([]string, error) {
	gitCmd := exec.Command("git", "diff", "--name-status", "macports/master...HEAD", "--")
	stdout, err := gitCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = gitCmd.Start(); err != nil {
		return nil, err
	}
	ports := make([]string, 0, 1)
	portsFound := make(map[string]bool)
	// Ignore hidden and _* top directories
	portGrep := regexp.MustCompile(
		`[AM]\t[^\._/][^/]*/([^/]+)/(Portfile|files/)`)
	renameGrep := regexp.MustCompile(
		`R[0-9]*\t[^\t]*\t[^\._/][^/]*/([^/]+)/(Portfile|files/)`)
	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		var match []string
		if match = portGrep.FindStringSubmatch(line); match == nil {
			continue
		} else if match = renameGrep.FindStringSubmatch(line); match == nil {
			continue
		}
		if _, ok := portsFound[match[1]]; !ok {
			portsFound[match[1]] = true
			ports = append(ports, match[1])
		}
	}
	if err = gitCmd.Wait(); err != nil {
		return nil, err
	}
	return ports, nil
}
