//go:build windows

package updater

import (
	"os/exec"
	"syscall"
)

func configureUpdateHelper(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
