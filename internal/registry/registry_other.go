//go:build !windows

package registry

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installIniPaths returns the candidates for the Epic Games Install.ini file on Linux/macOS.
func installIniPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".config", "Epic", "UnrealEngine", "Install.ini"),
		filepath.Join(home, ".config", "EpicGames", "UnrealEngine", "Install.ini"),
		"/etc/Epic/UnrealEngine/Install.ini",
	}
}

// parseInstallIni reads an Install.ini file and returns the [Installations] section
// as a map of association key → install path.
func parseInstallIni(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
	inSection := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSection = strings.EqualFold(line, "[Installations]")
			continue
		}
		if !inSection {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result, scanner.Err()
}

// GetEnginePathByAssociation resolves an engine association string (e.g. "UE_5.4", "5.4", or a GUID)
// to an absolute installation path by reading the Epic Install.ini on disk.
func GetEnginePathByAssociation(association string) (string, error) {
	for _, iniPath := range installIniPaths() {
		installs, err := parseInstallIni(iniPath)
		if err != nil {
			continue
		}

		// Try exact key match first (e.g. "UE_5.4")
		if path, ok := installs[association]; ok {
			return path, nil
		}

		// Try with "UE_" prefix stripped or added
		stripped := strings.TrimPrefix(association, "UE_")
		for key, path := range installs {
			if strings.EqualFold(key, association) ||
				strings.EqualFold(strings.TrimPrefix(key, "UE_"), stripped) {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("engine association %q not found in any Install.ini on this system", association)
}

func GetEngineInstallPath(version string) (string, error) {
	return GetEnginePathByAssociation(version)
}

func GetEngineSourcePath(sourcePath string) (string, error) {
	return GetEnginePathByAssociation(sourcePath)
}
