package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// AppSettings holds the persistent configuration for the Gorgeous Installer.
type AppSettings struct {
	SearchPaths       []string `json:"searchPaths"`
	LocalBinPath      string   `json:"localBinPath"`
	InstalledNatively     bool     `json:"installedNatively"`
	UprojectAssociated    bool     `json:"uprojectAssociated"`
	PrevUprojectCommand   string   `json:"prevUprojectCommand"`
}

// ConfigFilePath returns the path to the settings JSON file.
func ConfigFilePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "GorgeousThings", "Installer.json"), nil
}

// DefaultSettings returns the default configuration.
func DefaultSettings() *AppSettings {
	localBin := ""
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			localBin = filepath.Join(localAppData, "Programs", "GorgeousInstaller")
		}
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			localBin = filepath.Join(home, ".local", "bin")
		}
	}
	return &AppSettings{
		SearchPaths:       []string{},
		LocalBinPath:      localBin,
		InstalledNatively:   false,
		UprojectAssociated:  false,
		PrevUprojectCommand: "",
	}
}

// LoadSettings reads the settings from disk. Returns defaults if the file doesn't exist.
func LoadSettings() (*AppSettings, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return DefaultSettings(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), fmt.Errorf("failed to read settings file: %w", err)
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(data, settings); err != nil {
		return settings, fmt.Errorf("failed to parse settings file: %w", err)
	}

	// Update InstalledNatively based on actual presence
	settings.InstalledNatively = IsInstalledNatively()

	return settings, nil
}

// SaveSettings writes the settings to disk atomically.
func SaveSettings(settings *AppSettings) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp settings file: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to save settings file: %w", err)
	}

	return nil
}

// IsInstalledNatively checks if the desktop file/registry and binary exist.
func IsInstalledNatively() bool {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		binPath := filepath.Join(localAppData, "Programs", "GorgeousInstaller", "gorgeous-installer.exe")
		if _, err := os.Stat(binPath); err != nil {
			return false
		}
		// Check registry via reg query
		err := exec.Command("reg", "query", `HKCU\Software\Classes\.uproject`).Run()
		return err == nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	binPath := filepath.Join(home, ".local", "bin", "gorgeous-installer")
	desktopPath := filepath.Join(home, ".local", "share", "applications", "gorgeous-installer.desktop")

	_, errBin := os.Stat(binPath)
	_, errDesktop := os.Stat(desktopPath)

	return errBin == nil && errDesktop == nil
}
