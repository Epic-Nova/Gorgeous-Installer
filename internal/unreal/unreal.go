package unreal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gorgeous-installer/internal/registry"
)

// UProjectFile represents the structure of a .uproject file
type UProjectFile struct {
	FileVersion       int    `json:"FileVersion"`
	EngineAssociation string `json:"EngineAssociation"`
	Category          string `json:"Category"`
	Description       string `json:"Description"`
	Modules           []struct {
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

	association := strings.TrimSpace(uproject.EngineAssociation)
	if association == "" {
		return "", "", fmt.Errorf("EngineAssociation is empty in %s", uprojectPath)
	}

	enginePath, err = registry.GetEnginePathByAssociation(association)
	if err != nil {
		enginePath, err = findEngineInstallation(association)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve engine path for association %q: %w", association, err)
		}
	}

	version, err = NormalizeVersion(association)
	if err == nil {
		return version, enginePath, nil
	}

	version, err = readEngineVersionFromBuildFile(enginePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve semantic engine version from association %q and build metadata: %w", association, err)
	}

	return version, enginePath, nil
}

// LocateGorgeousPlugin retains compatibility for callers expecting the old API.
func LocateGorgeousPlugin(projectPath, enginePath string) (string, error) {
	return LocatePluginByName(projectPath, enginePath, "Gorgeous")
}

// LocatePluginByName finds the target plugin by matching the .uplugin file name
// in either project or engine plugin folders.
func LocatePluginByName(projectPath, enginePath, pluginName string) (string, error) {
	if pluginName == "" {
		return "", fmt.Errorf("plugin name is required")
	}

	projectRoot := projectPath
	if strings.HasSuffix(strings.ToLower(projectRoot), ".uproject") {
		projectRoot = filepath.Dir(projectRoot)
	}

	searchRoots := []string{filepath.Join(projectRoot, "Plugins")}
	if strings.TrimSpace(enginePath) != "" {
		searchRoots = append(searchRoots,
			filepath.Join(enginePath, "Engine", "Plugins", "Marketplace"),
			filepath.Join(enginePath, "Engine", "Plugins"),
		)
	}

	for _, root := range searchRoots {
		if pluginPath, err := findPluginByUPluginName(root, pluginName); err == nil {
			return pluginPath, nil
		}
	}

	return "", fmt.Errorf("plugin %q not found in project or engine plugin folders", pluginName)
}

func findPluginByUPluginName(pluginsRoot, pluginName string) (string, error) {
	if pluginsRoot == "" {
		return "", fmt.Errorf("empty plugins root")
	}

	if info, err := os.Stat(pluginsRoot); err != nil || !info.IsDir() {
		return "", fmt.Errorf("plugins root not found: %s", pluginsRoot)
	}

	// Fast-path: common direct plugin folder naming.
	direct := filepath.Join(pluginsRoot, pluginName)
	if isPluginWithMatchingUPlugin(direct, pluginName) {
		return direct, nil
	}

	var found string
	stop := errors.New("plugin-found")

	walkErr := filepath.WalkDir(pluginsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, relErr := filepath.Rel(pluginsRoot, path)
		if relErr == nil && rel != "." {
			if strings.Count(rel, string(os.PathSeparator)) > 5 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(d.Name()), ".uplugin") {
			return nil
		}

		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if strings.EqualFold(base, pluginName) {
			found = filepath.Dir(path)
			return stop
		}

		return nil
	})

	if walkErr != nil && !errors.Is(walkErr, stop) {
		return "", walkErr
	}
	if found == "" {
		return "", fmt.Errorf("plugin %q not found under %s", pluginName, pluginsRoot)
	}

	return found, nil
}

func isPluginWithMatchingUPlugin(pluginDir, pluginName string) bool {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".uplugin") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if strings.EqualFold(base, pluginName) {
			return true
		}
	}

	return false
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

type engineBuildVersion struct {
	MajorVersion int `json:"MajorVersion"`
	MinorVersion int `json:"MinorVersion"`
}

func readEngineVersionFromBuildFile(enginePath string) (string, error) {
	if enginePath == "" {
		return "", fmt.Errorf("engine path is empty")
	}

	buildVersionPath := filepath.Join(enginePath, "Engine", "Build", "Build.version")
	data, err := os.ReadFile(buildVersionPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", buildVersionPath, err)
	}

	var buildVersion engineBuildVersion
	if err := json.Unmarshal(data, &buildVersion); err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", buildVersionPath, err)
	}

	if buildVersion.MajorVersion <= 0 {
		return "", fmt.Errorf("invalid major version in %s", buildVersionPath)
	}

	return fmt.Sprintf("%d.%d", buildVersion.MajorVersion, buildVersion.MinorVersion), nil
}

// findEngineInstallation locates the UE installation directory for a given association/version.
func findEngineInstallation(association string) (string, error) {
	version, err := NormalizeVersion(association)
	if err != nil {
		return "", fmt.Errorf("could not normalize engine association %q: %w", association, err)
	}

	// Try standard installation paths
	standardPaths := []string{
		filepath.Join("C:\\Program Files", "Epic Games", "UE_"+version),
		filepath.Join("C:\\Program Files", "EpicGames", "UE_"+version),
		filepath.Join("D:\\Epic Games", "UE_"+version),
		filepath.Join("D:\\EpicGames", "UE_"+version),
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

func CheckProjectBinaries(projectPath string) bool {
	dir := projectPath
	if strings.HasSuffix(strings.ToLower(dir), ".uproject") {
		dir = filepath.Dir(dir)
	}

	// If no Source folder exists, it's a Blueprint project and needs no binaries
	sourceDir := filepath.Join(dir, "Source")
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return true // Ready to launch
	}

	platform := "Linux"
	if runtime.GOOS == "windows" {
		platform = "Win64"
	} else if runtime.GOOS == "darwin" {
		platform = "Mac"
	}

	projectModulesPath := filepath.Join(dir, "Binaries", platform, "UnrealEditor.modules")
	if _, err := os.Stat(projectModulesPath); os.IsNotExist(err) {
		return false // No modules file, definitely needs compile
	}

	projectData, err := os.ReadFile(projectModulesPath)
	if err != nil {
		return false
	}
	var projModules struct {
		BuildId string `json:"BuildId"`
	}
	if err := json.Unmarshal(projectData, &projModules); err != nil || projModules.BuildId == "" {
		return false
	}

	// Now get engine BuildId
	_, enginePath, err := GetEngineVersionFromProject(projectPath)
	if err != nil || enginePath == "" {
		// Can't resolve engine path, safer to trigger compile to let UBT figure it out
		return false
	}

	engineModulesPath := filepath.Join(enginePath, "Engine", "Binaries", platform, "UnrealEditor.modules")
	engineData, err := os.ReadFile(engineModulesPath)
	if err != nil {
		return true // If we can't find the engine modules (e.g. source build), trust the project ones
	}
	var engModules struct {
		BuildId string `json:"BuildId"`
	}
	if err := json.Unmarshal(engineData, &engModules); err != nil || engModules.BuildId == "" {
		return true
	}

	// Exact compatibility check
	return projModules.BuildId == engModules.BuildId
}
