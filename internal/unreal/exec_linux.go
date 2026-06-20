//go:build !windows

package unreal

import "os/exec"

// hideWindow does nothing on non-Windows platforms.
func hideWindow(cmd *exec.Cmd) {
	// Not needed on Linux/macOS
}
