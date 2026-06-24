package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AppSettings holds the persistent configuration for the Gorgeous Installer.
type AppSettings struct {
	SearchPaths         []string `json:"searchPaths"`
	LocalBinPath        string   `json:"localBinPath"`
	InstalledNatively   bool     `json:"installedNatively"`
	UprojectAssociated  bool     `json:"uprojectAssociated"`
	PrevUprojectCommand string   `json:"prevUprojectCommand"`
	DevMode             bool     `json:"devMode"`
	BinDevMode          bool     `json:"binDevMode"`
	ForceHTTP           bool     `json:"forceHTTP"`
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
		SearchPaths:         []string{},
		LocalBinPath:        localBin,
		InstalledNatively:   false,
		UprojectAssociated:  false,
		PrevUprojectCommand: "",
		DevMode:             false,
		BinDevMode:          false,
		ForceHTTP:           false,
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

// IsInstalledNatively checks if the current running executable is the natively installed binary.
func IsInstalledNatively() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}
	execPath, _ = filepath.EvalSymlinks(execPath)
	execPath = filepath.Clean(execPath)

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		binPath := filepath.Join(localAppData, "Programs", "GorgeousInstaller", "gorgeous-installer.exe")
		binPath = filepath.Clean(binPath)
		return strings.EqualFold(execPath, binPath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	binPath := filepath.Join(home, ".local", "bin", "gorgeous-installer")
	binPath = filepath.Clean(binPath)
	return execPath == binPath
}

// ErrorFilePath returns the path to the update_error.txt file.
func ErrorFilePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "GorgeousThings", "update_error.txt"), nil
}
