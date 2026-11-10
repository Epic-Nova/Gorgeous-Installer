//go:build !windows

package updater

import "os/exec"

func configureUpdateHelper(cmd *exec.Cmd) {}
