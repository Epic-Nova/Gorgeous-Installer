package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gorgeous-installer/internal/config"
)

func main() {
	packName := flag.String("pack", "", "Pack name (e.g., 'MyContentPack')")
	packType := flag.String("type", "content", "Pack type: content or code")
	packPath := flag.String("path", "", "Path to content/code pack directory")
	outputDir := flag.String("output", ".", "Output directory for config.json")
	version := flag.String("version", "1.0", "Pack version")

	flag.Parse()

	if *packName == "" || *packPath == "" {
		fmt.Println("Usage: go run cmd/builder/main.go -pack NAME -type [content|code] -path PACK_PATH [-output OUTPUT_DIR] [-version VERSION]")
		os.Exit(1)
	}

	// Create config
	cfg := &config.Config{
		PackName:    *packName,
		PackType:    *packType,
		InstallPath: "Content",
		AvailableVersions: []config.PackVersion{
			{
				Version: *version,
				Path:    *packPath,
				CheckSum: "", // Could be computed here
			},
		},
	}

	if *packType == "code" {
		cfg.InstallPath = "Source"
	}

	// Save config
	configPath := filepath.Join(*outputDir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
		os.Exit(1)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config created at: %s\n", configPath)
	fmt.Println("\nConfig contents:")
	fmt.Println(string(data))
}
