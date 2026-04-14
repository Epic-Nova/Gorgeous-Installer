package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorgeous-installer/internal/buildinfo"
	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/ui"
	"gorgeous-installer/internal/unreal"
)

type cliOptions struct {
	ProjectPath    string
	PackType       string
	PackVersion    string
	EngineVersion  string
	InstallAction  string
	ValidateSHA    bool
	ManifestSHA    string
	ExplicitCLIArg bool
}

func main() {
	// Mode and CLI flags
	cliMode := flag.Bool("cli", false, "Run in CLI mode")
	guiMode := flag.Bool("gui", false, "Run in GUI mode")
	projectPath := flag.String("project", "", "Path to .uproject file or project directory")
	packType := flag.String("type", "", "Pack type: content or code")
	packVersion := flag.String("version", "", "Pack version to install or validate")
	engineVersion := flag.String("engine-version", "", "Engine version used for pack selection (for install or SHA validation)")
	installAction := flag.String("action", "", "Install action: install, update, or reinstall")
	validateSHA := flag.Bool("validate-sha", false, "Validate pack files against a SHA manifest in CLI mode")
	shaFile := flag.String("sha-file", "", "Path to SHA/SHA256 manifest file")
	showBuildInfo := flag.Bool("version-info", false, "Print build metadata and exit")
	flag.Parse()

	if *showBuildInfo {
		fmt.Println(buildinfo.Summary())
		return
	}

	// Load configuration from embedded assets
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	opts := cliOptions{
		ProjectPath:    strings.TrimSpace(*projectPath),
		PackType:       strings.TrimSpace(*packType),
		PackVersion:    strings.TrimSpace(*packVersion),
		EngineVersion:  strings.TrimSpace(*engineVersion),
		InstallAction:  strings.TrimSpace(*installAction),
		ValidateSHA:    *validateSHA,
		ManifestSHA:    strings.TrimSpace(*shaFile),
		ExplicitCLIArg: *cliMode,
	}

	if opts.ProjectPath == "" && len(flag.Args()) > 0 {
		opts.ProjectPath = strings.TrimSpace(flag.Arg(0))
	}

	hasCLIInputs := opts.ValidateSHA ||
		opts.ProjectPath != "" ||
		opts.PackType != "" ||
		opts.PackVersion != "" ||
		opts.EngineVersion != "" ||
		opts.InstallAction != "" ||
		opts.ManifestSHA != ""

	if *guiMode {
		runGUIMode(cfg)
		return
	}

	if opts.ExplicitCLIArg || hasCLIInputs {
		runCLIMode(cfg, opts)
	} else {
		runGUIMode(cfg)
	}
}

func runCLIMode(cfg *config.Config, opts cliOptions) {
	if opts.ValidateSHA {
		runCLISHAValidation(cfg, opts)
		return
	}

	runCLIInstall(cfg, opts)
}

func runCLIInstall(cfg *config.Config, opts cliOptions) {
	if opts.ProjectPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --project is required for CLI installation")
		os.Exit(1)
	}

	packType := opts.PackType

	if packType == "" {
		packType = cfg.PackType
	}

	fmt.Printf("Starting installation in CLI mode\n")
	fmt.Printf("Project: %s\n", opts.ProjectPath)
	fmt.Printf("Pack Type: %s\n", packType)

	// Validate project path
	absPath, err := filepath.Abs(opts.ProjectPath)
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

	effectiveEngineVersion := ueVersion
	if opts.EngineVersion != "" {
		effectiveEngineVersion = opts.EngineVersion
		fmt.Printf("Engine Version Override: %s\n", effectiveEngineVersion)
	}

	selectedPack, selectErr := pickPackVersion(cfg, opts.PackVersion, effectiveEngineVersion)
	if selectErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to select pack version: %v\n", selectErr)
		os.Exit(1)
	}

	if selectedPack == nil {
		fmt.Fprintf(os.Stderr, "No compatible content pack found\n")
		os.Exit(1)
	}

	fmt.Printf("Selected Pack Version: %s\n", selectedPack.Version)

	// Locate Gorgeous plugin
	projectRoot := absPath
	if strings.HasSuffix(strings.ToLower(projectRoot), ".uproject") {
		projectRoot = filepath.Dir(projectRoot)
	}

	pluginPath, err := unreal.LocatePluginByName(projectRoot, enginePath, cfg.PluginName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to locate plugin %q: %v\n", cfg.PluginName, err)
		os.Exit(1)
	}

	fmt.Printf("Plugin Path: %s\n", pluginPath)

	// Perform installation
	inst := installer.NewInstaller(pluginPath, packType, selectedPack, cfg.InstallPath, absPath, enginePath)

	action := installer.InstallActionInstall
	if strings.TrimSpace(opts.InstallAction) != "" {
		action, err = parseInstallAction(opts.InstallAction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid --action value: %v\n", err)
			os.Exit(1)
		}
	} else if plan, planErr := inst.BuildInstallPlan(); planErr == nil {
		action = plan.Action
		fmt.Printf("Detected Action: %s\n", action)
		if action == installer.InstallActionUpdate {
			fmt.Printf("Changed Files: %d\n", len(plan.ChangedFiles))
		}
	}

	inst.SetInstallAction(action)
	fmt.Printf("Running Action: %s\n", action)

	if err := inst.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Installation completed successfully!")
}

func runGUIMode(cfg *config.Config) {
	ui.NewGUIApp(cfg).Run()
}

func runCLISHAValidation(cfg *config.Config, opts cliOptions) {
	if strings.TrimSpace(opts.PackVersion) == "" && strings.TrimSpace(opts.EngineVersion) == "" {
		fmt.Fprintln(os.Stderr, "SHA validation requires --version or --engine-version")
		os.Exit(1)
	}

	selectedPack, err := pickPackVersion(cfg, opts.PackVersion, opts.EngineVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to select pack version for SHA validation: %v\n", err)
		os.Exit(1)
	}

	manifestPath := opts.ManifestSHA
	if strings.TrimSpace(manifestPath) == "" {
		resolved, found := installer.ResolvePackSHAManifestPath(selectedPack)
		if !found || strings.TrimSpace(resolved) == "" {
			fmt.Fprintf(os.Stderr, "No SHA manifest specified and no configured shaFile found for version %s\n", selectedPack.Version)
			os.Exit(1)
		}

		manifestPath = resolved
	}

	fmt.Printf("Validating SHA in CLI mode\n")
	fmt.Printf("Pack Version: %s\n", selectedPack.Version)
	fmt.Printf("Manifest: %s\n", manifestPath)

	report, err := installer.ValidatePackSHA(selectedPack, manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SHA validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Entries: %d\n", report.TotalEntries)
	fmt.Printf("Matched: %d\n", report.MatchedFiles)
	fmt.Printf("Missing: %d\n", len(report.MissingFiles))
	fmt.Printf("Mismatched: %d\n", len(report.Mismatches))

	for _, missing := range report.MissingFiles {
		fmt.Printf("MISSING: %s\n", missing)
	}
	for _, mismatch := range report.Mismatches {
		fmt.Printf("MISMATCH: %s\n", mismatch.FilePath)
	}

	if !report.IsValid() {
		os.Exit(1)
	}

	fmt.Println("SHA validation completed successfully")
}

func parseInstallAction(value string) (installer.InstallAction, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "install":
		return installer.InstallActionInstall, nil
	case "update":
		return installer.InstallActionUpdate, nil
	case "reinstall":
		return installer.InstallActionReinstall, nil
	default:
		return "", fmt.Errorf("expected install, update, or reinstall")
	}
}

func pickPackVersion(cfg *config.Config, requestedVersion, requestedEngineVersion string) (*config.PackVersion, error) {
	if strings.TrimSpace(requestedVersion) != "" {
		return findPackVersion(cfg, requestedVersion)
	}

	if strings.TrimSpace(requestedEngineVersion) != "" {
		selected := selectOptimalPackVersion(cfg, requestedEngineVersion)
		if selected == nil {
			return nil, fmt.Errorf("no compatible pack version found for engine version %s", requestedEngineVersion)
		}

		return selected, nil
	}

	if len(cfg.AvailableVersions) == 0 {
		return nil, fmt.Errorf("no pack versions configured")
	}

	return &cfg.AvailableVersions[0], nil
}

func findPackVersion(cfg *config.Config, version string) (*config.PackVersion, error) {
	trimmed := strings.TrimSpace(version)
	for i := range cfg.AvailableVersions {
		if cfg.AvailableVersions[i].Version == trimmed {
			return &cfg.AvailableVersions[i], nil
		}
	}

	return nil, fmt.Errorf("pack version %s not found", version)
}

func selectOptimalPackVersion(cfg *config.Config, ueVersion string) *config.PackVersion {
	normalizedVersion := ueVersion
	if normalized, err := unreal.NormalizeVersion(ueVersion); err == nil {
		normalizedVersion = normalized
	}

	var bestMatch *config.PackVersion
	foundExact := false

	for _, pv := range cfg.AvailableVersions {
		if pv.Version == normalizedVersion {
			bestMatch = &pv
			foundExact = true
			break
		}
	}

	// If no exact match, prefer older versions
	if !foundExact && len(cfg.AvailableVersions) > 0 {
		for _, pv := range cfg.AvailableVersions {
			if isVersionOlder(pv.Version, normalizedVersion) {
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
