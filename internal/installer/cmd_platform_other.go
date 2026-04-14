//go:build !windows

package installer

import "os/exec"

func configureCommandForPlatform(_ *exec.Cmd) {
	// No platform-specific command attributes needed.
}
