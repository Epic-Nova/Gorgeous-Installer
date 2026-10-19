//go:build windows

package installer

import (
	"os/exec"
	"syscall"
)

const STILL_ACTIVE = 0x00000103

func configureCommandForPlatform(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func isProcessRunning(pid int) bool {
	const STANDARD_RIGHTS_REQUIRED = 0x000F0000
	const SYNCHRONIZE = 0x00100000
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	handle, err := syscall.OpenProcess(STANDARD_RIGHTS_REQUIRED|SYNCHRONIZE|PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	err = syscall.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}
	return exitCode == STILL_ACTIVE
}
