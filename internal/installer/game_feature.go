package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// gameFeatureDescriptor validates a content-only Game Feature plugin payload.
// The descriptor must opt into activation; installing assets alone does not execute them.
type gameFeatureDescriptor struct {
	Type                       string            `json:"Type"`
	BuiltInInitialFeatureState string            `json:"BuiltInInitialFeatureState"`
	Modules                    []json.RawMessage `json:"Modules"`
	Plugins                    []struct {
		Name    string `json:"Name"`
		Enabled bool   `json:"Enabled"`
	} `json:"Plugins"`
}

func gameFeatureName(files []string) (string, error) {
	var descriptor string
	for _, rel := range files {
		if strings.EqualFold(filepath.Ext(rel), ".uplugin") && !strings.Contains(filepath.ToSlash(rel), "/") {
			if descriptor != "" {
				return "", fmt.Errorf("game feature must have exactly one root .uplugin")
			}
			descriptor = rel
		}
		first := strings.Split(filepath.ToSlash(rel), "/")[0]
		if first == "Binaries" || first == "Intermediate" || first == "Saved" || first == ".git" {
			return "", fmt.Errorf("generated plugin files are not allowed: %s", rel)
		}
		if strings.HasSuffix(strings.ToLower(rel), ".uproject") {
			return "", fmt.Errorf("project files are not allowed in a game feature: %s", rel)
		}
	}
	if descriptor == "" {
		return "", fmt.Errorf("game feature requires one root .uplugin descriptor")
	}
	return strings.TrimSuffix(descriptor, filepath.Ext(descriptor)), nil
}

func validateGameFeaturePayload(root string, files []string) (string, error) {
	name, err := gameFeatureName(files)
	if err != nil {
		return "", err
	}
	data, err := readPackFileData(root, name+".uplugin")
	if err != nil {
		return "", err
	}
	var d gameFeatureDescriptor
	if err = json.Unmarshal(data, &d); err != nil {
		return "", fmt.Errorf("invalid game feature descriptor: %w", err)
	}
	if !strings.EqualFold(d.Type, "GameFeature") {
		return "", fmt.Errorf("plugin Type must be GameFeature")
	}
	if len(d.Modules) > 0 {
		return "", fmt.Errorf("game-feature-example supports content-only plugins; code modules require a separate build flow")
	}
	if !strings.EqualFold(d.BuiltInInitialFeatureState, "Active") {
		return "", fmt.Errorf("BuiltInInitialFeatureState must be Active for automatic activation")
	}
	hasData := false
	for _, rel := range files {
		if strings.HasPrefix(filepath.ToSlash(rel), "Content/") && strings.HasSuffix(strings.ToLower(rel), ".uasset") {
			hasData = true
		}
	}
	if !hasData {
		return "", fmt.Errorf("game feature requires Content assets, including GameFeatureData")
	}
	return name, nil
}

func (i *Installer) installGameFeature() error {
	plan, err := i.BuildInstallPlan()
	if err != nil {
		return err
	}
	files, err := listPackFiles(plan.SourceRoot)
	if err != nil {
		return err
	}
	name, err := validateGameFeaturePayload(plan.SourceRoot, files)
	if err != nil {
		return err
	}
	projectFile, err := resolveProjectFilePath(i.ProjectPath)
	if err != nil {
		return err
	}
	// Never silently replace a project's existing plugin descriptor with a different name.
	if filepath.Base(plan.DestinationRoot) != name {
		return fmt.Errorf("game feature destination mismatch")
	}
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return err
	}
	var project map[string]json.RawMessage
	if err = json.Unmarshal(data, &project); err != nil {
		return fmt.Errorf("invalid .uproject: %w", err)
	}
	var plugins []map[string]json.RawMessage
	if raw := project["Plugins"]; len(raw) > 0 {
		if err = json.Unmarshal(raw, &plugins); err != nil {
			return fmt.Errorf("invalid project Plugins: %w", err)
		}
	}
	found := false
	for _, p := range plugins {
		var existing string
		_ = json.Unmarshal(p["Name"], &existing)
		if strings.EqualFold(existing, name) {
			p["Enabled"] = json.RawMessage("true")
			found = true
		}
	}
	if !found {
		n, _ := json.Marshal(name)
		plugins = append(plugins, map[string]json.RawMessage{"Name": n, "Enabled": json.RawMessage("true")})
	}
	// Preflight descriptor conflicts before writing any content.
	targetDescriptor := filepath.Join(plan.DestinationRoot, name+".uplugin")
	if _, err := os.Stat(targetDescriptor); err == nil {
		existing, readErr := os.ReadFile(targetDescriptor)
		if readErr != nil {
			return readErr
		}
		source, readErr := readPackFileData(plan.SourceRoot, name+".uplugin")
		if readErr != nil {
			return readErr
		}
		if string(existing) != string(source) && i.effectiveAction(plan) == InstallActionInstall {
			return fmt.Errorf("plugin %s already exists; choose update or reinstall", name)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	action := i.effectiveAction(plan)
	filesToCopy := files
	if action == InstallActionUpdate {
		filesToCopy = plan.ChangedFiles
	}
	if err := copyPackFiles(plan.SourceRoot, plan.DestinationRoot, filesToCopy); err != nil {
		return err
	}
	updated, err := json.MarshalIndent(plugins, "  ", "  ")
	if err != nil {
		return err
	}
	project["Plugins"] = updated
	output, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}
	output = append(output, '\n')
	if err := os.WriteFile(projectFile, output, 0644); err != nil {
		return fmt.Errorf("plugin copied but project registration failed: %w", err)
	}
	i.logf("Game Feature %s installed and enabled. Restart Unreal Editor to discover and activate it.", name)
	return nil
}
