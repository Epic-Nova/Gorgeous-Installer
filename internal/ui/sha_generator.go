package ui

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateSHA256Manifest creates a sha256 checksum manifest for all files in packDir.
// It writes the manifest to shaFilePath with relative paths based on packDir.
func generateSHA256Manifest(packDir string, shaFilePath string) error {
	var lines []string

	err := filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		hash := sha256.Sum256(data)
		rel, err := filepath.Rel(packDir, path)
		if err != nil {
			return err
		}

		// Use forward slashes for cross-platform compatibility in the manifest
		relSlash := filepath.ToSlash(rel)
		lines = append(lines, fmt.Sprintf("%x  %s", hash, relSlash))
		return nil
	})

	if err != nil {
		return err
	}

	// Write the lines to the output file
	outContent := strings.Join(lines, "\n")
	if len(lines) > 0 {
		outContent += "\n"
	}
	return os.WriteFile(shaFilePath, []byte(outContent), 0644)
}
