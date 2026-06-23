//go:build !windows

package installer

import (
	"os"
	"os/exec"
	"syscall"
)

func configureCommandForPlatform(_ *exec.Cmd) {
	// No platform-specific command attributes needed.
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
