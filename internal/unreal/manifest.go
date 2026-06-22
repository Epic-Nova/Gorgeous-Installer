package unreal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gorgeous-installer/internal/api"
)

// SystemManifest represents the JSON manifest required for GT ecosystem packs.
type SystemManifest struct {
	ID           string   `json:"SystemId"`
	Version      string   `json:"Version"`
	Name         string   `json:"Name"`
	Description  string   `json:"Description"`
	IsCoreSystem bool     `json:"bIsCoreSystem"`
	PayloadPaths []string `json:"PayloadPaths"`
}

func RecreateMissingManifests(projectPath string) {
	systems, err := api.GetSystems()
	if err != nil {
		return // Silently return if offline or API error
	}

	_, enginePath, err := GetEngineVersionFromProject(projectPath)
	if err != nil || enginePath == "" {
		return
	}

	for _, sys := range systems {
		if sys.IsCoreSystem {
			continue // Core systems don't have user-facing manifests in the same way, or we ignore them
		}

		pluginPath, err := LocatePluginByName(projectPath, enginePath, sys.TargetPluginName)
		if err != nil || pluginPath == "" {
			continue
		}

		// To avoid deep scanning the entire plugin repeatedly, we assume the manifest 
		// should be placed in the preferred payload path. If it's not there, we'll just write it.
		// A more robust approach would be to cache all SystemManifest.json files in the project.
		// For now, let's check the preferred payload path directly.
		
		var targetFolder string
		if len(sys.SourcePaths) > 0 {
			targetFolder = filepath.Join(pluginPath, sys.SourcePaths[0])
		} else if len(sys.ContentPaths) > 0 {
			targetFolder = filepath.Join(pluginPath, sys.ContentPaths[0])
		} else {
			targetFolder = pluginPath
		}

		manifestPath := filepath.Join(targetFolder, "SystemManifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			
			// Recreate PayloadPaths from SourcePaths and ContentPaths
			var payloadPaths []string
			payloadPaths = append(payloadPaths, sys.SourcePaths...)
			payloadPaths = append(payloadPaths, sys.ContentPaths...)

			manifest := SystemManifest{
				ID:           sys.SystemId,
				Version:      sys.Version,
				Name:         sys.DisplayName,
				Description:  sys.Description,
				IsCoreSystem: sys.IsCoreSystem,
				PayloadPaths: payloadPaths,
			}

			data, err := json.MarshalIndent(manifest, "", "  ")
			if err == nil {
				os.MkdirAll(targetFolder, 0755)
				os.WriteFile(manifestPath, data, 0644)
				fmt.Printf("[Boot] Recreated missing SystemManifest.json for %s at %s\n", sys.SystemId, manifestPath)
			}
		}
	}
}
