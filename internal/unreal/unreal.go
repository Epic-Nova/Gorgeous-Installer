package unreal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gorgeous-installer/internal/registry"
)

// UProjectFile represents the structure of a .uproject file
type UProjectFile struct {
	FileVersion      int    `json:"FileVersion"`
	EngineAssociation string `json:"EngineAssociation"`
	Category         string `json:"Category"`
	Description      string `json:"Description"`
	Modules          []struct {
		Name string `json:"Name"`
	} `json:"Modules"`
}

// GetEngineVersionFromProject extracts the engine version from a .uproject file
// Returns the engine version string and the engine installation path
func GetEngineVersionFromProject(projectPath string) (version, enginePath string, err error) {
	// Find the .uproject file
	uprojectPath, err := findUProjectFile(projectPath)
	if err != nil {
		return "", "", fmt.Errorf("could not find .uproject file: %w", err)
	}

	// Read and parse the .uproject file
	data, err := os.ReadFile(uprojectPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read uproject file: %w", err)
	}

	var uproject UProjectFile
	if err := json.Unmarshal(data, &uproject); err != nil {
		return "", "", fmt.Errorf("failed to parse uproject file: %w", err)
	}

	version = uproject.EngineAssociation

	// Handle source builds (versions wrapped in {})
	if strings.HasPrefix(version, "{") && strings.HasSuffix(version, "}") {
		// Source build - look up in registry
		sourcePath, err := registry.GetEngineSourcePath(version)
		if err != nil {
			return "", "", fmt.Errorf("failed to locate source build engine: %w", err)
		}
		return version, sourcePath, nil
	}

	// Standard engine installation - find by version
	enginePath, err = findEngineInstallation(version)
	return version, enginePath, err
}

// LocateGorgeousPlugin finds the Gorgeous plugin in either the project or engine directory
func LocateGorgeousPlugin(projectPath, enginePath string) (string, error) {
	// First, try to find it in the project's Plugins directory
	projectPluginsPath := filepath.Join(projectPath, "Plugins", "Gorgeous")
	if isValidPluginPath(projectPluginsPath) {
		return projectPluginsPath, nil
	}

	// Try to find it in the engine's Plugins directory
	enginePluginsPath := filepath.Join(enginePath, "Engine", "Plugins", "Marketplace", "Gorgeous")
	if isValidPluginPath(enginePluginsPath) {
		return enginePluginsPath, nil
	}

	// Try alternative paths
	altEnginePluginsPath := filepath.Join(enginePath, "Engine", "Plugins", "Gorgeous")
	if isValidPluginPath(altEnginePluginsPath) {
		return altEnginePluginsPath, nil
	}

	return "", fmt.Errorf("gorgeous plugin not found in project or engine directories")
}

// findUProjectFile locates the .uproject file in the given directory
func findUProjectFile(projectPath string) (string, error) {
	// If projectPath is already a .uproject file, return it
	if strings.HasSuffix(strings.ToLower(projectPath), ".uproject") {
		return projectPath, nil
	}

	// Search for .uproject file in the directory
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".uproject") {
			return filepath.Join(projectPath, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .uproject file found in %s", projectPath)
}

// isValidPluginPath checks if a plugin path exists and contains uplugin file
func isValidPluginPath(pluginPath string) bool {
	info, err := os.Stat(pluginPath)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for .uplugin file
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".uplugin") {
			return true
		}
	}

	return false
}

// findEngineInstallation locates the UE installation directory for a given version
func findEngineInstallation(version string) (string, error) {
	// Try standard installation paths
	standardPaths := []string{
		filepath.Join("C:\\Program Files", "Epic Games", "UE_"+version),
		filepath.Join("C:\\Program Files", "EpicGames", "UE_"+version),
		filepath.Join(os.Getenv("APPDATA"), "..", "Local", "EpicGamesLauncher", "Saved", "InstallationInfo.json"),
	}

	for _, path := range standardPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}

	// On Windows, try to get from registry
	enginePath, err := registry.GetEngineInstallPath(version)
	if err == nil {
		return enginePath, nil
	}

	return "", fmt.Errorf("UE %s installation not found", version)
}

// NormalizeVersion converts version string to standardized format (X.X)
func NormalizeVersion(v string) (string, error) {
	// Handle source builds
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
		v = strings.TrimPrefix(strings.TrimSuffix(v, "}"), "{")
	}

	// Extract version numbers using regex
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(v)
	if len(matches) < 3 {
		return "", fmt.Errorf("invalid version format: %s", v)
	}

	return matches[1] + "." + matches[2], nil
}
