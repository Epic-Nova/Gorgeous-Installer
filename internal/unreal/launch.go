package unreal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// LaunchUnrealEditor launches the Unreal Editor with the given project
func LaunchUnrealEditor(projectPath string) error {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	_, enginePath, err := GetEngineVersionFromProject(absPath)
	if err != nil {
		return fmt.Errorf("failed to get engine version: %w", err)
	}

	var editorExe string
	if runtime.GOOS == "windows" {
		editorExe = filepath.Join(enginePath, "Engine", "Binaries", "Win64", "UnrealEditor.exe")
	} else if runtime.GOOS == "darwin" {
		editorExe = filepath.Join(enginePath, "Engine", "Binaries", "Mac", "UnrealEditor.app", "Contents", "MacOS", "UnrealEditor")
	} else {
		editorExe = filepath.Join(enginePath, "Engine", "Binaries", "Linux", "UnrealEditor")
	}

	if _, err := os.Stat(editorExe); os.IsNotExist(err) {
		return fmt.Errorf("editor executable not found at %s", editorExe)
	}

	cmd := exec.Command(editorExe, absPath, "-BypassGorgeousHook")

	// Start the process detached so it survives the installer closing
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start editor process: %w", err)
	}

	return nil
}
