package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceDirectory locates this installer rather than assuming the launcher's current directory is source.
func SourceDirectory(executable, workingDirectory string) (string, error) {
	for _, start := range []string{filepath.Dir(executable), workingDirectory} {
		if start == "" || (executable == "" && start == ".") {
			continue
		}
		dir, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					fields := strings.Fields(line)
					if len(fields) >= 2 && fields[0] == "module" && strings.Trim(fields[1], "\"") == "gorgeous-installer" {
						return dir, nil
					}
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("could not locate Gorgeous Installer source; run from its source checkout or rebuild the standalone installation")
}
