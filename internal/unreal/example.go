package unreal

import (
	"encoding/json"
	"fmt"
	"os"
)

// GetExampleEngineVersion does not require a locally installed engine for asset-only copies.
// Source-build GUID associations still use normal engine resolution and build metadata.
func GetExampleEngineVersion(projectPath string) (string, string, error) {
	file, err := findUProjectFile(projectPath)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", "", err
	}
	var project UProjectFile
	if err := json.Unmarshal(data, &project); err != nil {
		return "", "", fmt.Errorf("invalid .uproject: %w", err)
	}
	if version, err := NormalizeVersion(project.EngineAssociation); err == nil {
		return version, "", nil
	}
	return GetEngineVersionFromProject(projectPath)
}
