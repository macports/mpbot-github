package ci

import (
	"os/exec"
)

// Only deactivate ports to save time when a dependency
// is needed across builds. It should be able to avoid
// conflicts.
func DeactivateAllPorts() {
	deactivateCmd := exec.Command("port", "-fp", "deactivate", "active")
	deactivateCmd.Run()
}
