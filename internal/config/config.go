package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the installer configuration
type Config struct {
	PackName          string         `json:"packName"`
	PackType          string         `json:"packType"` // "content" or "code"
	InstallPath       string         `json:"installPath"`
	AvailableVersions []PackVersion  `json:"availableVersions"`
	ContentData       []byte         `json:"-"` // Embedded content/code pack
	MetadataPath      string         `json:"metadataPath"`
}

// PackVersion represents a specific pack version for a UE version
type PackVersion struct {
	Version  string `json:"version"`
	Path     string `json:"path"`
	CheckSum string `json:"checksum"`
}

// LoadConfig loads configuration from config.json in the current executable directory
// The config should be packaged with the installer
func LoadConfig() (*Config, error) {
	// Try to load config from embedded resources or same directory as executable
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")

	// If config doesn't exist at exe level, try current working directory
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		// Return a default config if not found
		return &Config{
			PackName:    "Gorgeous Pack",
			PackType:    "content",
			InstallPath: "Content",
			AvailableVersions: []PackVersion{
				{Version: "5.4", Path: "packs/5.4"},
				{Version: "5.3", Path: "packs/5.3"},
				{Version: "5.2", Path: "packs/5.2"},
				{Version: "4.27", Path: "packs/4.27"},
			},
		}, nil
	}

	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to a JSON file
func (c *Config) SaveConfig(filepath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}
