package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// GetEngineInstallPath retrieves the installation path for a specific UE version from the registry
func GetEngineInstallPath(version string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("registry lookup only supported on Windows")
	}

	// Try HKLM first
	path, err := getRegistryPath(registry.LOCAL_MACHINE, version)
	if err == nil {
		return path, nil
	}

	// Try HKCU as fallback
	path, err = getRegistryPath(registry.CURRENT_USER, version)
	return path, err
}

// GetEngineSourcePath retrieves the source engine path from registry
// For source builds identified by {SourcePath}
func GetEngineSourcePath(sourcePath string) (string, error) {
	if runtime.GOOS != "windows" {
		// On non-Windows, try environment variable or default paths
		return getSourcePathNonWindows(sourcePath)
	}

	// Parse the source path reference
	cleanPath := strings.TrimPrefix(strings.TrimSuffix(sourcePath, "}"), "{")

	// Check if it's already a valid path
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return cleanPath, nil
	}

	// Try to find in registry
	path, err := getSourceRegistryPath(registry.LOCAL_MACHINE, sourcePath)
	if err == nil {
		return path, nil
	}

	path, err = getSourceRegistryPath(registry.CURRENT_USER, sourcePath)
	return path, err
}

// getRegistryPath retrieves engine path from a registry hive
func getRegistryPath(hive registry.Key, version string) (string, error) {
	// Epic Games launcher registry paths
	registryPaths := []string{
		`Software\Epic Games\Unreal Engine\Builds`,
		`Software\EpicGames\Unreal Engine\Builds`,
		`Software\EpicGames\Engine`,
	}

	for _, regPath := range registryPaths {
		k, err := registry.OpenKey(hive, regPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		defer k.Close()

		// Try to get the value for this version
		path, _, err := k.GetStringValue(version)
		if err == nil && path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("engine version %s not found in registry", version)
}

// getSourceRegistryPath retrieves source engine path from registry
func getSourceRegistryPath(hive registry.Key, sourcePath string) (string, error) {
	cleanPath := strings.TrimPrefix(strings.TrimSuffix(sourcePath, "}"), "{")

	registryPaths := []string{
		`Software\Epic Games\Unreal Engine\SourceBuilds`,
		`Software\EpicGames\Unreal Engine\SourceBuilds`,
	}

	for _, regPath := range registryPaths {
		k, err := registry.OpenKey(hive, regPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		defer k.Close()

		path, _, err := k.GetStringValue(cleanPath)
		if err == nil && path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("source build path %s not found in registry", sourcePath)
}

// getSourcePathNonWindows handles source path lookup on macOS and Linux
func getSourcePathNonWindows(sourcePath string) (string, error) {
	cleanPath := strings.TrimPrefix(strings.TrimSuffix(sourcePath, "}"), "{")

	// Check if it's already a valid path
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return cleanPath, nil
	}

	// Try common locations
	homedir, _ := os.UserHomeDir()

	commonPaths := []string{
		filepath.Join(homedir, "UnrealEngine", cleanPath),
		filepath.Join(homedir, "UE", cleanPath),
		filepath.Join("/opt", "UnrealEngine", cleanPath),
	}

	for _, path := range commonPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}

	return cleanPath, nil // Return the path as-is if we can't find it
}

func init() {
	// For non-Windows platforms, registry package won't be used
}
