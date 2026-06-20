package installer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	bundle "gorgeous-installer"
	"gorgeous-installer/internal/config"
)

// Installer handles the installation of content or code packs
type Installer struct {
	PluginPath   string
	PackType     string
	PackVersion  *config.PackVersion
	InstallPath  string
	ProjectPath  string
	EnginePath   string
	PackContent  []byte
	SourceDir    string
	RecompileOnly bool
	Action       InstallAction
	StatusLogger func(string, ...any)
	RunContext   context.Context
}

// NewInstaller creates a new installer instance
func NewInstaller(pluginPath, packType string, packVersion *config.PackVersion, installPath, projectPath, enginePath string) *Installer {
	inst := &Installer{
		PluginPath:  pluginPath,
		PackType:    packType,
		PackVersion: packVersion,
		InstallPath: installPath,
		ProjectPath: projectPath,
		EnginePath:  enginePath,
	}
	inst.StatusLogger = func(format string, args ...any) {
		fmt.Printf(format+"\n", args...)
	}

	return inst
}

// SetPackContent sets the embedded pack content
func (i *Installer) SetPackContent(content []byte) {
	i.PackContent = content
}

// SetStatusLogger configures where installer status lines are written.
func (i *Installer) SetStatusLogger(logger func(string, ...any)) {
	if logger != nil {
		i.StatusLogger = logger
	}
}

// SetRunContext configures cancellation for long-running installer operations.
func (i *Installer) SetRunContext(ctx context.Context) {
	if ctx != nil {
		i.RunContext = ctx
	}
}

// SetInstallAction configures whether install should run as install, update, or reinstall.
func (i *Installer) SetInstallAction(action InstallAction) {
	switch action {
	case InstallActionInstall, InstallActionUpdate, InstallActionReinstall:
		i.Action = action
	default:
		i.Action = InstallActionInstall
	}
}

func (i *Installer) commandContext() context.Context {
	if i.RunContext != nil {
		return i.RunContext
	}

	return context.Background()
}

func (i *Installer) logf(format string, args ...any) {
	if i.StatusLogger != nil {
		i.StatusLogger(format, args...)
	}
}

func (i *Installer) effectiveAction(plan *InstallPlan) InstallAction {
	switch i.Action {
	case InstallActionInstall, InstallActionUpdate, InstallActionReinstall:
		return i.Action
	}

	if plan != nil {
		switch plan.Action {
		case InstallActionInstall, InstallActionUpdate, InstallActionReinstall:
			return plan.Action
		}
	}

	return InstallActionInstall
}

// Install performs the installation based on pack type
func (i *Installer) Install() error {
	// If we're fully packless and just recompiling, don't even check PackType
	if i.PackVersion == nil && i.RecompileOnly {
		i.logf("Packless recompile-only mode — invoking UnrealBuildTool directly")
		if err := i.recompilePlugin(); err != nil {
			return fmt.Errorf("plugin recompilation failed: %w", err)
		}
		i.logf("Recompile complete")
		return nil
	}

	switch i.PackType {
	case "content":
		return i.installContentPack()
	case "code":
		return i.installCodePack()
	case "hybrid":
		return i.installHybridPack()
	default:
		return fmt.Errorf("unknown pack type: %s", i.PackType)
	}
}

// installContentPack installs a content pack to the plugin's Content directory
func (i *Installer) installContentPack() error {
	plan, err := i.BuildInstallPlan()
	if err != nil {
		return err
	}

	contentDir := plan.DestinationRoot
	action := i.effectiveAction(plan)

	// Create content directory if it doesn't exist
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}

	// Extract pack content to the Content directory
	// For now, we'll copy from the configured path or use embedded content
	installPath := i.PackVersion.Path

	i.logf("Install action: %s", action)
	i.logf("Installing content pack from: %s", installPath)
	i.logf("Installing to: %s", contentDir)
	i.logf("Pack version: %s", plan.PackVersion)

	if action == InstallActionUpdate {
		if len(plan.ChangedFiles) == 0 {
			i.logf("No changed files detected for update; nothing to copy")
			return nil
		}

		i.logf("Updating %d changed files", len(plan.ChangedFiles))
		if err := copyPackFiles(installPath, contentDir, plan.ChangedFiles); err != nil {
			return fmt.Errorf("failed to copy updated content files: %w", err)
		}

		i.logf("Content pack updated successfully")
		return nil
	}

	// Copy contents from pack to plugin content directory
	if err := copyPackDirectory(installPath, contentDir); err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	i.logf("Content pack installed successfully")
	return nil
}

// installCodePack installs a code pack and recompiles the plugin
func (i *Installer) installCodePack() error {
	// Recompile-only mode: skip pack selection and file operations entirely.
	// This is used by --verify-compatibility and --recompile-only to rebuild
	// the plugin without touching any source files.
	if i.RecompileOnly {
		i.logf("Recompile-only mode — skipping file copy, invoking UnrealBuildTool directly")
		if err := i.recompilePlugin(); err != nil {
			return fmt.Errorf("plugin recompilation failed: %w", err)
		}
		i.logf("Recompile complete")
		return nil
	}

	plan, err := i.BuildInstallPlan()
	if err != nil {
		return err
	}

	sourceDir := plan.DestinationRoot
	action := i.effectiveAction(plan)

	// Create source directory if needed
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	installPath := i.PackVersion.Path
	i.logf("Install action: %s", action)
	i.logf("Installing code pack from: %s", installPath)
	i.logf("Installing to: %s", sourceDir)
	i.logf("Pack version: %s", plan.PackVersion)

	if action == InstallActionUpdate {
		if len(plan.ChangedFiles) == 0 {
			i.logf("No changed files detected for update; skipping code copy and recompile")
			return nil
		}

		i.logf("Updating %d changed code files", len(plan.ChangedFiles))
		if err := copyPackFiles(installPath, sourceDir, plan.ChangedFiles); err != nil {
			return fmt.Errorf("failed to copy updated code files: %w", err)
		}
	} else {
		// Copy all code files
		if err := copyPackDirectory(installPath, sourceDir); err != nil {
			return fmt.Errorf("failed to copy code: %w", err)
		}
	}

	i.logf("Code pack installed, attempting plugin recompilation")

	// Recompile the plugin
	if err := i.recompilePlugin(); err != nil {
		return fmt.Errorf("plugin recompilation failed: %w. Try to rebuild the project from sln project manually", err)
	}

	i.logf("Code pack installed successfully")
	return nil
}

// installHybridPack installs a hybrid pack (content + code) to the plugin root
func (i *Installer) installHybridPack() error {
	plan, err := i.BuildInstallPlan()
	if err != nil {
		return err
	}

	sourceDir := plan.DestinationRoot
	action := i.effectiveAction(plan)

	// Create root directory if needed
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create hybrid directory: %w", err)
	}

	installPath := i.PackVersion.Path
	i.logf("Install action: %s", action)
	i.logf("Installing hybrid pack from: %s", installPath)
	i.logf("Installing to: %s", sourceDir)
	i.logf("Pack version: %s", plan.PackVersion)

	if action == InstallActionUpdate {
		if len(plan.ChangedFiles) == 0 {
			i.logf("No changed files detected for update; skipping hybrid copy and recompile")
			return nil
		}

		i.logf("Updating %d changed hybrid files", len(plan.ChangedFiles))
		if err := copyPackFiles(installPath, sourceDir, plan.ChangedFiles); err != nil {
			return fmt.Errorf("failed to copy updated hybrid files: %w", err)
		}
	} else {
		// Copy all hybrid files
		if err := copyPackDirectory(installPath, sourceDir); err != nil {
			return fmt.Errorf("failed to copy hybrid content: %w", err)
		}
	}

	i.logf("Hybrid pack installed, attempting plugin recompilation")

	// Recompile the plugin
	if err := i.recompilePlugin(); err != nil {
		return fmt.Errorf("plugin recompilation failed: %w. Try to rebuild the project from sln project manually", err)
	}

	i.logf("Hybrid pack installed successfully")
	return nil
}


// recompilePlugin attempts to recompile the plugin using UnrealBuildTool
func (i *Installer) recompilePlugin() error {
	if strings.TrimSpace(i.EnginePath) == "" {
		return fmt.Errorf("engine path is required to compile code packs")
	}

	// Find the .uplugin file to compile the correct plugin.
	upluginPath, err := findUpluginFile(i.PluginPath)
	if err != nil {
		return fmt.Errorf("failed to find .uplugin file: %w", err)
	}

	pluginName := strings.TrimSuffix(filepath.Base(upluginPath), filepath.Ext(upluginPath))
	i.logf("Compiling plugin: %s", pluginName)
	i.logf("Engine path: %s", i.EnginePath)

	projectFile, projectErr := resolveProjectFilePath(i.ProjectPath)
	if projectErr != nil {
		i.logf("Could not resolve .uproject for compile: %v", projectErr)
		return fmt.Errorf("could not resolve .uproject for compile: %w", projectErr)
	}

	targetName := editorTargetFromProject(projectFile)

	// Select platform string for UBT
	platform := "Win64"
	switch runtime.GOOS {
	case "linux":
		platform = "Linux"
	case "darwin":
		platform = "Mac"
	}

	// On Linux/Mac, prefer the shell wrapper (RunUAT.sh / UBT.sh) so we don't
	// depend on a specific .NET runtime version being installed.
	ubtPath, ubtErr := findUnrealBuildToolExecutable(i.EnginePath)
	if ubtErr != nil {
		i.logf("UnrealBuildTool not found: %v", ubtErr)
		return fmt.Errorf("UnrealBuildTool not found: %w", ubtErr)
	}

	var cmd *exec.Cmd
	if runtime.GOOS != "windows" {
		// On Linux/Mac, invoke via the shell wrapper to avoid .NET version pinning.
		shWrapper := filepath.Join(i.EnginePath, "Engine", "Build", "BatchFiles", "Linux", "Build.sh")
		if runtime.GOOS == "darwin" {
			shWrapper = filepath.Join(i.EnginePath, "Engine", "Build", "BatchFiles", "Mac", "Build.sh")
		}
		if _, statErr := os.Stat(shWrapper); statErr == nil {
			i.logf("Using shell Build.sh wrapper: %s", shWrapper)
			cmd = exec.CommandContext(
				i.commandContext(),
				"/bin/bash",
				shWrapper,
				targetName,
				platform,
				"Development",
				"-Project="+projectFile,
				"-WaitMutex",
				"-Progress",
			)
			cmd.Dir = i.EnginePath
		} else {
			i.logf("Build.sh not found at %s, falling back to UBT executable", shWrapper)
		}
	}

	if cmd == nil {
		// Fallback: direct UBT executable invocation (Windows or when Build.sh is missing)
		i.logf("Using UnrealBuildTool executable: %s", ubtPath)
		cmd = exec.CommandContext(
			i.commandContext(),
			ubtPath,
			targetName,
			platform,
			"Development",
			"-Project="+projectFile,
			"-WaitMutex",
			"-FromMSBuild",
			"-Progress",
		)
		cmd.Dir = i.EnginePath
	}

	cmd.Env = append(os.Environ(), "GORGEOUS_SKIP_INSTALLER_BUILD=1")

	if runErr := i.runCommandWithLog(cmd); runErr != nil {
		i.logf("Recompile failed: %v", runErr)
		return fmt.Errorf("plugin recompilation failed: %w", runErr)
	}

	return nil
}

func (i *Installer) runCommandWithLog(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("build command is nil")
	}

	configureCommandForPlatform(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture stdout: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to capture stderr: %w", err)
	}

	i.logf("Running build command: %s", strings.Join(cmd.Args, " "))

	if err := cmd.Start(); err != nil {
		if ctxErr := i.commandContext().Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("failed to start build command: %w", err)
	}

	stream := func(reader io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			i.logf("%s", line)
		}

		if scanErr := scanner.Err(); scanErr != nil {
			i.logf("Build log stream error: %v", scanErr)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go stream(stdoutPipe, &wg)
	go stream(stderrPipe, &wg)

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if ctxErr := i.commandContext().Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}

	return nil
}

// findUpluginFile locates the .uplugin file in the plugin directory
func findUpluginFile(pluginPath string) (string, error) {
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return "", fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".uplugin") {
			return filepath.Join(pluginPath, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .uplugin file found in %s", pluginPath)
}

func findAutomationTool(enginePath string) (string, error) {
	enginePath = strings.TrimSpace(enginePath)
	if enginePath == "" {
		return "", fmt.Errorf("engine path is empty")
	}

	candidates := []string{}
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			filepath.Join(enginePath, "Engine", "Build", "BatchFiles", "RunUAT.bat"),
			filepath.Join(enginePath, "Engine", "Build", "BatchFiles", "RunUAT.cmd"),
		)
	case "darwin", "linux":
		candidates = append(candidates,
			filepath.Join(enginePath, "Engine", "Build", "BatchFiles", "RunUAT.sh"),
		)
	default:
		return "", fmt.Errorf("unsupported platform for code-pack compilation: %s", runtime.GOOS)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("RunUAT not found in engine path %s", enginePath)
}

func findUnrealBuildToolDLL(enginePath string) (string, error) {
	enginePath = strings.TrimSpace(enginePath)
	if enginePath == "" {
		return "", fmt.Errorf("engine path is empty")
	}

	candidates := []string{
		filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool", "UnrealBuildTool.dll"),
		filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool.dll"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("UnrealBuildTool.dll not found in engine path %s", enginePath)
}

func findUnrealBuildToolExecutable(enginePath string) (string, error) {
	enginePath = strings.TrimSpace(enginePath)
	if enginePath == "" {
		return "", fmt.Errorf("engine path is empty")
	}

	candidates := []string{}
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool", "UnrealBuildTool.exe"),
			filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool.exe"),
			filepath.Join(enginePath, "Engine", "Binaries", "Win64", "UnrealBuildTool.exe"),
		)
	case "darwin", "linux":
		candidates = append(candidates,
			filepath.Join(enginePath, "Engine", "Binaries", "DotNET", "UnrealBuildTool", "UnrealBuildTool"),
		)
	default:
		return "", fmt.Errorf("unsupported platform for UnrealBuildTool lookup: %s", runtime.GOOS)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("UnrealBuildTool not found in engine path %s", enginePath)
}

func editorTargetFromProject(projectFile string) string {
	name := strings.TrimSuffix(filepath.Base(projectFile), filepath.Ext(projectFile))
	if strings.HasSuffix(strings.ToLower(name), "editor") {
		return name
	}

	return name + "Editor"
}

func resolveProjectFilePath(projectPath string) (string, error) {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return "", fmt.Errorf("project path is empty")
	}

	if strings.HasSuffix(strings.ToLower(projectPath), ".uproject") {
		if info, err := os.Stat(projectPath); err == nil && !info.IsDir() {
			return projectPath, nil
		}
		return "", fmt.Errorf("uproject file not found: %s", projectPath)
	}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".uproject") {
			return filepath.Join(projectPath, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .uproject file found in %s", projectPath)
}

func (i *Installer) resolveInstallDir(baseDir, folderType string) (string, error) {
	subPath, err := normalizeInstallSubPath(i.InstallPath, folderType)
	if err != nil {
		return "", err
	}

	if subPath == "" {
		return baseDir, nil
	}

	return filepath.Join(baseDir, subPath), nil
}

func normalizeInstallSubPath(installPath, folderType string) (string, error) {
	clean := strings.TrimSpace(installPath)
	if clean == "" {
		return "", nil
	}

	clean = strings.ReplaceAll(clean, "\\", "/")
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	clean = path.Clean(clean)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid install path traversal: %s", installPath)
	}

	parts := strings.Split(clean, "/")
	if len(parts) > 0 {
		if strings.EqualFold(parts[0], folderType) {
			parts = parts[1:]
		} else if strings.EqualFold(folderType, "source") && strings.EqualFold(parts[0], "content") {
			// Code packs should not end up in Source/Content unless explicitly nested beyond the root folder.
			parts = parts[1:]
		}
	}

	joined := strings.Join(parts, "/")
	if joined == "" || joined == "." {
		return "", nil
	}

	return filepath.FromSlash(joined), nil
}

// copyDirectory recursively copies a directory
func copyDirectory(src, dst string) error {
	// Ensure source exists
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory does not exist: %w", err)
	}

	if !info.IsDir() {
		return copyFile(src, dst)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if isSHAControlFile(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			if err := copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	if isSHAControlFile(filepath.Base(src)) {
		return nil
	}

	// Handle if dst is a directory
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Preserve file permissions
	if info, err := os.Stat(src); err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}

func copyPackDirectory(src, dst string) error {
	norm := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(src)), "./")

	if err := copyEmbeddedDirectory(norm, dst); err == nil {
		return nil
	}

	// Fallback for development mode where assets are read from disk.
	return copyDirectory(src, dst)
}

func copyEmbeddedDirectory(src, dst string) error {
	if src == "" || src == "." {
		return fmt.Errorf("invalid embedded source path: %q", src)
	}
	if strings.HasPrefix(src, "../") {
		return fmt.Errorf("invalid embedded source path traversal: %q", src)
	}

	entries, err := bundle.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, entry := range entries {
		srcChild := path.Join(src, entry.Name())
		dstChild := filepath.Join(dst, entry.Name())
		if isSHAControlFile(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			if err := copyEmbeddedDirectory(srcChild, dstChild); err != nil {
				return err
			}
			continue
		}

		data, readErr := bundle.ReadFile(srcChild)
		if readErr != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", srcChild, readErr)
		}

		if err := os.WriteFile(dstChild, data, 0644); err != nil {
			return fmt.Errorf("failed to write destination file %s: %w", dstChild, err)
		}
	}

	return nil
}
