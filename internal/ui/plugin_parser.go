package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// UPlugin represents a minimal .uplugin file structure
type UPlugin struct {
	FriendlyName string `json:"FriendlyName"`
	VersionName  string `json:"VersionName"`
}

// ParsePluginInfo reads the .uplugin file and extracts name and version
func ParsePluginInfo(pluginDir string) (string, string, string, error) {
	matches, err := filepath.Glob(filepath.Join(pluginDir, "*.uplugin"))
	if err != nil || len(matches) == 0 {
		return "", "", "", fmt.Errorf("no .uplugin file found in %s", pluginDir)
	}

	upluginPath := matches[0]
	pluginID := strings.TrimSuffix(filepath.Base(upluginPath), ".uplugin")

	data, err := os.ReadFile(upluginPath)
	if err != nil {
		return "", "", "", err
	}

	var p UPlugin
	if err := json.Unmarshal(data, &p); err != nil {
		return "", "", "", err
	}

	return pluginID, p.FriendlyName, p.VersionName, nil
}

// FindMinimumCoreVersion scans Source/ for Module files and extracts the minimum core version
func FindMinimumCoreVersion(pluginDir string) string {
	var moduleFiles []string

	// Check Source directory
	sourceDir := filepath.Join(pluginDir, "Source")
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if (ext == ".h" || ext == ".cpp") && strings.Contains(info.Name(), "Module") {
				moduleFiles = append(moduleFiles, path)
			}
		}
		return nil
	})

	// Regex to match: return 100;
	re := regexp.MustCompile(`(?m)GetMinimumRequiredCoreVersion[^\{]*\{[^}]*return\s+(\d+)\s*;`)
	
	for _, file := range moduleFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		content := string(data)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			// Found the integer, convert to semantic version
			var versionInt int
			fmt.Sscanf(matches[1], "%d", &versionInt)
			
			major := versionInt / 100
			minor := (versionInt % 100) / 10
			patch := versionInt % 10
			
			return fmt.Sprintf("%d.%d.%d", major, minor, patch)
		}
	}
	
	// Fallback to searching all C++ files if not found in Module files
	reFallback := regexp.MustCompile(`(?m)GetMinimumRequiredCoreVersion\s*\(\)\s*(?:const)?\s*(?:override)?\s*\{\s*return\s+(\d+)\s*;\s*\}`)
	var allFiles []string
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && (filepath.Ext(path) == ".h" || filepath.Ext(path) == ".cpp") {
			allFiles = append(allFiles, path)
		}
		return nil
	})
	
	for _, file := range allFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		content := string(data)
		matches := reFallback.FindStringSubmatch(content)
		if len(matches) > 1 {
			var versionInt int
			fmt.Sscanf(matches[1], "%d", &versionInt)
			
			major := versionInt / 100
			minor := (versionInt % 100) / 10
			patch := versionInt % 10
			
			return fmt.Sprintf("%d.%d.%d", major, minor, patch)
		}
	}
	
	return ""
}
