package installer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gorgeous-installer/internal/config"
)

// Installer handles the installation of content or code packs
type Installer struct {
	PluginPath  string
	PackType    string
	PackVersion *config.PackVersion
	PackContent []byte
}

// NewInstaller creates a new installer instance
func NewInstaller(pluginPath, packType string, packVersion *config.PackVersion) *Installer {
	return &Installer{
		PluginPath:  pluginPath,
		PackType:    packType,
		PackVersion: packVersion,
	}
}

// SetPackContent sets the embedded pack content
func (i *Installer) SetPackContent(content []byte) {
	i.PackContent = content
}

// Install performs the installation based on pack type
func (i *Installer) Install() error {
	switch i.PackType {
	case "content":
		return i.installContentPack()
	case "code":
		return i.installCodePack()
	default:
		return fmt.Errorf("unknown pack type: %s", i.PackType)
	}
}

// installContentPack installs a content pack to the plugin's Content directory
func (i *Installer) installContentPack() error {
	contentDir := filepath.Join(i.PluginPath, "Content")

	// Create content directory if it doesn't exist
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}

	// Extract pack content to the Content directory
	// For now, we'll copy from the configured path or use embedded content
	installPath := i.PackVersion.Path

	fmt.Printf("Installing content pack from: %s\n", installPath)
	fmt.Printf("Installing to: %s\n", contentDir)

	// Copy contents from pack to plugin content directory
	if err := copyDirectory(installPath, contentDir); err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	fmt.Println("Content pack installed successfully")
	return nil
}

// installCodePack installs a code pack and recompiles the plugin
func (i *Installer) installCodePack() error {
	// Code packs go to Source folder + configured path
	sourceDir := filepath.Join(i.PluginPath, "Source")

	// Create source directory if needed
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	installPath := i.PackVersion.Path
	fmt.Printf("Installing code pack from: %s\n", installPath)
	fmt.Printf("Installing to: %s\n", sourceDir)

	// Copy code files
	if err := copyDirectory(installPath, sourceDir); err != nil {
		return fmt.Errorf("failed to copy code: %w", err)
	}

	fmt.Println("Code pack installed, attempting plugin recompilation")

	// Recompile the plugin
	if err := i.recompilePlugin(); err != nil {
		fmt.Printf("Warning: Plugin recompilation failed: %v\n", err)
		// Don't fail the installation, the code is copied
	}

	fmt.Println("Code pack installed successfully")
	return nil
}

// recompilePlugin attempts to recompile the plugin using UnrealBuildTool
func (i *Installer) recompilePlugin() error {
	// Find the .uplugin file to get plugin name
	upluginPath, err := findUpluginFile(i.PluginPath)
	if err != nil {
		return fmt.Errorf("failed to find .uplugin file: %w", err)
	}

	pluginName := strings.TrimSuffix(filepath.Base(upluginPath), ".uplugin")

	// Try to find UnrealBuildTool
	ubtPath, err := findUnrealBuildTool()
	if err != nil {
		return fmt.Errorf("UnrealBuildTool not found: %w", err)
	}

	fmt.Printf("Using UnrealBuildTool: %s\n", ubtPath)
	fmt.Printf("Recompiling plugin: %s\n", pluginName)

	// Run UBT to rebuild the plugin
	cmd := exec.Command(ubtPath, "Development", "Win64", "-Project="+i.PluginPath, "-TargetType=Editor")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("plugin recompilation failed: %w", err)
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

// findUnrealBuildTool attempts to locate the UnrealBuildTool executable
func findUnrealBuildTool() (string, error) {
	// Try relative to the engine installation
	possiblePaths := []string{
		"Engine\\Binaries\\Win64\\UnrealBuildTool.exe",
		"..\\..\\Engine\\Binaries\\Win64\\UnrealBuildTool.exe",
		"Tools\\UnrealBuildTool\\Binaries\\Win64\\UnrealBuildTool.exe",
	}

	for _, relativePath := range possiblePaths {
		absPath, _ := filepath.Abs(relativePath)
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("UnrealBuildTool not found")
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
