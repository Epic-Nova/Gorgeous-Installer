//go:build !windows

package registry

import (
	"fmt"
)

// Structural mock definitions for compilation safety on Linux
type Key int

const (
	CURRENT_USER  Key = 0
	LOCAL_MACHINE Key = 1
)

func lookupBuildMap(hive Key, association string) (string, error) {
	return "", fmt.Errorf("windows registry optimization maps are not available on this operating system")
}

func lookupInstalledDirectoryKeys(hive Key, association string) (string, error) {
	return "", fmt.Errorf("windows application install directory keys are not available on this operating system")
}

func findStandardInstallPath(association string) (string, error) {
	return "", fmt.Errorf("standard windows drive letters do not exist on this operating system")
}
