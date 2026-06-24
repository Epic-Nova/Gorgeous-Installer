package unreal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OpenProject launches the Unreal Editor for the given project in the background.
func OpenProject(uprojectPath string) error {
	absPath, err := filepath.Abs(uprojectPath)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	_, enginePath, err := GetEngineVersionFromProject(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect engine: %w", err)
	}

	editorBin := filepath.Join(enginePath, "Engine", "Binaries", "Linux", "UnrealEditor")
	if _, err := os.Stat(editorBin); err != nil {
		return fmt.Errorf("UnrealEditor not found at %s", editorBin)
	}

	cmd := exec.Command(editorBin, absPath)
	hideWindow(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start UnrealEditor: %w", err)
	}

	// Detach process
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// GenerateProjectFiles runs the Unreal Engine project file generator.
func GenerateProjectFiles(ctx context.Context, uprojectPath string, logFn func(string, ...any)) error {
	absPath, err := filepath.Abs(uprojectPath)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	_, enginePath, err := GetEngineVersionFromProject(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect engine: %w", err)
	}

	generatorScript := filepath.Join(enginePath, "Engine", "Build", "BatchFiles", "Linux", "GenerateProjectFiles.sh")
	if runtime.GOOS == "windows" {
		generatorScript = filepath.Join(enginePath, "Engine", "Build", "BatchFiles", "GenerateProjectFiles.bat")
	}

	if _, err := os.Stat(generatorScript); err != nil {
		return fmt.Errorf("generator script not found at %s", generatorScript)
	}

	logFn("Starting GenerateProjectFiles...")

	cmd := exec.CommandContext(ctx, generatorScript, "-project="+absPath, "-game")
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(absPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("GenerateProjectFiles failed to start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			logFn("%s", line)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("GenerateProjectFiles failed: %w", err)
	}

	logFn("GenerateProjectFiles completed successfully.")
	return nil
}

// BuildProject invokes the UnrealBuildTool to compile the project.
func BuildProject(ctx context.Context, uprojectPath string, logFn func(string, ...any)) error {
	absPath, err := filepath.Abs(uprojectPath)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	_, enginePath, err := GetEngineVersionFromProject(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect engine: %w", err)
	}

	projectName := strings.TrimSuffix(filepath.Base(absPath), ".uproject")

	ubtDll := filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool", "UnrealBuildTool.dll")
	if _, err := os.Stat(ubtDll); err != nil {
		return fmt.Errorf("UnrealBuildTool.dll not found at %s", ubtDll)
	}

	dotNetDir := filepath.Join(enginePath, "Engine", "Binaries", "ThirdParty", "DotNet")
	var bundledDotnet string
	_ = filepath.Walk(dotNetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			name := strings.ToLower(info.Name())
			if name == "dotnet" || name == "dotnet.exe" {
				bundledDotnet = path
				return filepath.SkipDir
			}
		}
		return nil
	})

	dotnetExe := "dotnet"
	if bundledDotnet != "" {
		dotnetExe = bundledDotnet
	}

	platform := "Linux"
	if runtime.GOOS == "windows" {
		platform = "Win64"
	}

	cmd := exec.CommandContext(ctx, dotnetExe, ubtDll, projectName+"Editor", platform, "Development", "-Project="+absPath, "-buildscw")
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(absPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("build failed to start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			logFn("%s", line)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	logFn("Build completed successfully.")
	return nil
}

// KillUBT forcefully kills any running instances of UnrealBuildTool
func KillUBT() {
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "dotnet.exe").Run()
		exec.Command("taskkill", "/F", "/IM", "UnrealBuildTool.exe").Run()
	} else {
		exec.Command("pkill", "-9", "-f", "UnrealBuildTool").Run()
		exec.Command("pkill", "-9", "-f", "dotnet").Run()
	}
}
