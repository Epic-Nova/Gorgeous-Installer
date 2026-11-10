//go:build !windows

package selfinstall

import (
	"fmt"
	"path/filepath"
)

func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }
func currentPlatform() (platform, error) {
	return nil, fmt.Errorf("Windows shell installation is unavailable on this platform")
}
func Uninstall(string) (bool, error) {
	return false, fmt.Errorf("Windows shell uninstall is unavailable on this platform")
}
