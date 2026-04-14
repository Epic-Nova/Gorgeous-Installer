//go:build windows

package installer

import (
	"os/exec"
	"syscall"
)

func configureCommandForPlatform(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
