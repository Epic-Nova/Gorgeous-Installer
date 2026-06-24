package config

import (
	"encoding/json"
	"os"

	bundle "gorgeous-installer"
)

// Config holds the installer configuration
type Config struct {
	PackName          string        `json:"packName"`
	PluginName        string        `json:"pluginName"`
	PackType          string        `json:"packType"` // "content" or "code"
	InstallPath       string        `json:"installPath"`
	AvailableVersions []PackVersion `json:"availableVersions"`
	ContentData       []byte        `json:"-"` // Embedded content/code pack
}

// PackVersion represents a specific pack version for a UE version
type PackVersion struct {
	Version         string   `json:"version"`
	Path            string   `json:"path"`
	SHAFile         string   `json:"shaFile,omitempty"`
	CheckSum        string   `json:"checksum"`
	SupportedVersions []string `json:"supportedVersions,omitempty"`
}

// LoadConfig loads configuration from embedded config.json.
func LoadConfig() (*Config, error) {
	configData, err := bundle.ReadFile("config.json")
	if err != nil {
		// Return a default config if embedded config is unavailable.
		return &Config{
			PackName:    "Gorgeous Pack",
			PluginName:  "Gorgeous",
			PackType:    "content",
			InstallPath: "Content",
			AvailableVersions: []PackVersion{
				{Version: "5.7", Path: "packs/5.7/content"},
				{Version: "5.6", Path: "packs/5.6/content"},
				{Version: "5.5", Path: "packs/5.5/content"},
				{Version: "5.4", Path: "packs/5.4/content"},
				{Version: "5.3", Path: "packs/5.3/content"},
				{Version: "5.2", Path: "packs/5.2/content"},
				{Version: "5.1", Path: "packs/5.1/content"},
			},
		}, nil
	}

	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, err
	}

	if cfg.PluginName == "" {
		cfg.PluginName = "Gorgeous"
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
