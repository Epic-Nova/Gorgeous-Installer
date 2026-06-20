//go:build windows

package unreal

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents the command from popping up a black console window on Windows.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
