package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/ui"
	"gorgeous-installer/internal/unreal"
	// "gorgeous-installer/internal/ui" // GUI temporarily disabled

)

func main() {
	// CLI mode flags
	cliMode := flag.Bool("cli", false, "Run in CLI mode")
	projectPath := flag.String("project", "", "Path to .uproject file or project directory")
	packType := flag.String("type", "", "Pack type: content or code")
	flag.Parse()

	// Load configuration from embedded assets
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if *cliMode {
		// CLI installation mode
		runCLIMode(cfg, *projectPath, *packType)
	} else {
		// GUI mode - animated branded interface
		runGUIMode(cfg)
	}
}

func runCLIMode(cfg *config.Config, projectPath, packType string) {
	if projectPath == "" {
		fmt.Println("Error: --project flag is required in CLI mode")
		os.Exit(1)
	}

	fmt.Printf("Starting installation in CLI mode\n")
	fmt.Printf("Project: %s\n", projectPath)
	fmt.Printf("Pack Type: %s\n", packType)

	// Validate project path
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid project path: %v\n", err)
		os.Exit(1)
	}

	// Determine UE version from uproject
	ueVersion, enginePath, err := unreal.GetEngineVersionFromProject(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine engine version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Detected UE Version: %s\n", ueVersion)
	fmt.Printf("Engine Path: %s\n", enginePath)

	// Find the best matching content pack version
	selectedPack := selectOptimalPackVersion(cfg, ueVersion)
	if selectedPack == nil {
		fmt.Fprintf(os.Stderr, "No compatible content pack found\n")
		os.Exit(1)
	}

	fmt.Printf("Selected Pack Version: %s\n", selectedPack.Version)

	// Locate Gorgeous plugin
	pluginPath, err := unreal.LocateGorgeousPlugin(absPath, enginePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to locate Gorgeous plugin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Plugin Path: %s\n", pluginPath)

	// Perform installation
	inst := installer.NewInstaller(pluginPath, packType, selectedPack)
	if err := inst.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Installation completed successfully!")
}

func runGUIMode(cfg *config.Config) {
	guiApp := ui.NewGUIApp(cfg)
	guiApp.Run()
}

func selectOptimalPackVersion(cfg *config.Config, ueVersion string) *config.PackVersion {
	var bestMatch *config.PackVersion
	foundExact := false

	for _, pv := range cfg.AvailableVersions {
		if pv.Version == ueVersion {
			bestMatch = &pv
			foundExact = true
			break
		}
	}

	// If no exact match, prefer older versions
	if !foundExact && len(cfg.AvailableVersions) > 0 {
		for _, pv := range cfg.AvailableVersions {
			if isVersionOlder(pv.Version, ueVersion) {
				if bestMatch == nil || isVersionOlder(bestMatch.Version, pv.Version) {
					bestMatch = &pv
				}
			}
		}
	}

	return bestMatch
}

func isVersionOlder(v1, v2 string) bool {
	// Simple version comparison (X.X format)
	var v1Major, v1Minor, v2Major, v2Minor int
	fmt.Sscanf(v1, "%d.%d", &v1Major, &v1Minor)
	fmt.Sscanf(v2, "%d.%d", &v2Major, &v2Minor)

	if v1Major != v2Major {
		return v1Major < v2Major
	}
	return v1Minor < v2Minor
}
