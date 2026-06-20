//go:build windows

package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)`)

// GetEngineInstallPath retrieves the installation path for a specific UE version from the registry
func GetEngineInstallPath(version string) (string, error) {
	return GetEnginePathByAssociation(version)
}

// GetEngineSourcePath retrieves the source engine path from registry
// For source builds identified by {SourcePath}
func GetEngineSourcePath(sourcePath string) (string, error) {
	return GetEnginePathByAssociation(sourcePath)
}

// GetEnginePathByAssociation resolves launcher and source builds from an EngineAssociation value.
func GetEnginePathByAssociation(association string) (string, error) {
	association = strings.TrimSpace(association)
	if association == "" {
		return "", fmt.Errorf("engine association is empty")
	}

	if direct, ok := directPathFromAssociation(association); ok {
		return direct, nil
	}

	if runtime.GOOS != "windows" {
		return getEnginePathNonWindows(association)
	}

	if path, err := lookupBuildMap(registry.CURRENT_USER, association); err == nil {
		return path, nil
	}
	if path, err := lookupBuildMap(registry.LOCAL_MACHINE, association); err == nil {
		return path, nil
	}

	if path, err := lookupInstalledDirectoryKeys(registry.LOCAL_MACHINE, association); err == nil {
		return path, nil
	}
	if path, err := lookupInstalledDirectoryKeys(registry.CURRENT_USER, association); err == nil {
		return path, nil
	}

	if path, err := findStandardInstallPath(association); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("engine path for association %q not found", association)
}

func lookupBuildMap(hive registry.Key, association string) (string, error) {
	registryPaths := []string{
		`Software\Epic Games\Unreal Engine\Builds`,
		`Software\EpicGames\Unreal Engine\Builds`,
		`Software\Epic Games\Unreal Engine\SourceBuilds`,
		`Software\EpicGames\Unreal Engine\SourceBuilds`,
	}

	candidates := associationLookupCandidates(association)

	for _, regPath := range registryPaths {
		k, err := registry.OpenKey(hive, regPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		path, lookupErr := lookupStringValueAny(k, candidates)
		k.Close()
		if lookupErr == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("engine association %q not found in build maps", association)
}

func lookupInstalledDirectoryKeys(hive registry.Key, association string) (string, error) {
	version, ok := parseVersion(association)
	if !ok {
		return "", fmt.Errorf("association %q does not include semantic version", association)
	}

	versionKeyCandidates := []string{version, "UE_" + version}
	basePaths := []string{
		`Software\Epic Games\Unreal Engine\%s`,
		`Software\EpicGames\Unreal Engine\%s`,
		`Software\WOW6432Node\Epic Games\Unreal Engine\%s`,
		`Software\WOW6432Node\EpicGames\Unreal Engine\%s`,
	}
	valueCandidates := []string{"InstalledDirectory", "InstalledLocation", "InstallDir", "InstalledDir"}

	for _, versionKey := range versionKeyCandidates {
		for _, base := range basePaths {
			keyPath := fmt.Sprintf(base, versionKey)

			k, err := registry.OpenKey(hive, keyPath, registry.QUERY_VALUE)
			if err != nil {
				continue
			}

			path, lookupErr := lookupStringValueAny(k, valueCandidates)
			k.Close()
			if lookupErr == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("installed directory key not found for association %q", association)
}

func lookupStringValueAny(k registry.Key, candidates []string) (string, error) {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		value, _, err := k.GetStringValue(candidate)
		if err != nil {
			continue
		}

		if existing, ok := existingDirectory(value); ok {
			return existing, nil
		}
	}

	normalizedTargets := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		norm := normalizeAssociationToken(candidate)
		if norm != "" {
			normalizedTargets[norm] = struct{}{}
		}
	}

	valueNames, err := k.ReadValueNames(-1)
	if err != nil {
		return "", err
	}

	for _, name := range valueNames {
		if _, ok := normalizedTargets[normalizeAssociationToken(name)]; !ok {
			continue
		}

		value, _, readErr := k.GetStringValue(name)
		if readErr != nil {
			continue
		}

		if existing, ok := existingDirectory(value); ok {
			return existing, nil
		}
	}

	return "", fmt.Errorf("no matching string values found")
}

func associationLookupCandidates(association string) []string {
	association = strings.TrimSpace(association)
	inner := unwrapAssociation(association)

	candidates := []string{association, inner}
	if inner != "" {
		candidates = append(candidates, "{"+inner+"}")
	}

	if version, ok := parseVersion(association); ok {
		candidates = append(candidates, version, "UE_"+version, "UE"+version)
	}

	return uniqueNonEmpty(candidates)
}

func directPathFromAssociation(association string) (string, bool) {
	if existing, ok := existingDirectory(association); ok {
		return existing, true
	}

	inner := unwrapAssociation(association)
	if existing, ok := existingDirectory(inner); ok {
		return existing, true
	}

	return "", false
}

func getEnginePathNonWindows(association string) (string, error) {
	inner := unwrapAssociation(association)
	homeDir, _ := os.UserHomeDir()

	commonPaths := []string{}
	if version, ok := parseVersion(association); ok {
		switch runtime.GOOS {
		case "darwin":
			commonPaths = append(commonPaths,
				filepath.Join("/Users", "Shared", "Epic Games", "UE_"+version),
				filepath.Join("/Users", "Shared", "EpicGames", "UE_"+version),
			)
		case "linux":
			commonPaths = append(commonPaths,
				filepath.Join(homeDir, "EpicGames", "UE_"+version),
				filepath.Join(homeDir, ".local", "share", "Epic", "UE_"+version),
				filepath.Join("/opt", "EpicGames", "UE_"+version),
			)
		}
	}

	if inner != "" {
		commonPaths = append(commonPaths,
			filepath.Join(homeDir, "UnrealEngine", inner),
			filepath.Join(homeDir, "UE", inner),
			filepath.Join("/opt", "UnrealEngine", inner),
		)
	}

	for _, candidate := range uniqueNonEmpty(commonPaths) {
		if existing, ok := existingDirectory(candidate); ok {
			return existing, nil
		}
	}

	return "", fmt.Errorf("engine path for association %q not found on %s", association, runtime.GOOS)
}

func findStandardInstallPath(association string) (string, error) {
	version, ok := parseVersion(association)
	if !ok {
		return "", fmt.Errorf("association %q does not include semantic version", association)
	}

	candidates := []string{
		filepath.Join("C:\\Program Files", "Epic Games", "UE_"+version),
		filepath.Join("C:\\Program Files", "EpicGames", "UE_"+version),
		filepath.Join("D:\\Epic Games", "UE_"+version),
		filepath.Join("D:\\EpicGames", "UE_"+version),
	}

	for _, candidate := range candidates {
		if existing, ok := existingDirectory(candidate); ok {
			return existing, nil
		}
	}

	return "", fmt.Errorf("standard install path not found for version %s", version)
}

func parseVersion(value string) (string, bool) {
	matches := versionPattern.FindStringSubmatch(value)
	if len(matches) < 3 {
		return "", false
	}

	return matches[1] + "." + matches[2], true
}

func normalizeAssociationToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimSuffix(value, "}"), "{")
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func unwrapAssociation(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(value, "}"), "{"))
}

func existingDirectory(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}

	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", false
	}

	return clean, true
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
