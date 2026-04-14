package installer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	bundle "gorgeous-installer"
	"gorgeous-installer/internal/config"
)

// SHAMismatch represents one file with a checksum mismatch.
type SHAMismatch struct {
	FilePath string
	Expected string
	Actual   string
}

// SHAValidationReport summarizes checksum validation results for one pack version.
type SHAValidationReport struct {
	PackVersion  string
	ManifestPath string
	TotalEntries int
	MatchedFiles int
	MissingFiles []string
	Mismatches   []SHAMismatch
}

// IsValid returns true when no missing or mismatching files were found.
func (r *SHAValidationReport) IsValid() bool {
	if r == nil {
		return false
	}

	return len(r.MissingFiles) == 0 && len(r.Mismatches) == 0 && r.TotalEntries > 0
}

// ResolvePackSHAManifestPath locates a checksum manifest for the provided pack version.
func ResolvePackSHAManifestPath(packVersion *config.PackVersion) (string, bool) {
	if packVersion == nil {
		return "", false
	}

	if configured := strings.TrimSpace(packVersion.SHAFile); configured != "" {
		if shaAssetExists(configured) {
			return configured, true
		}

		return configured, false
	}

	root := normalizePackRoot(packVersion.Path)
	if root == "" {
		return "", false
	}

	candidates := []string{
		root + ".sha256",
		root + ".sha",
		path.Join(root, "sha256.txt"),
		path.Join(root, "manifest.sha256"),
	}

	for _, candidate := range candidates {
		if shaAssetExists(candidate) {
			return candidate, true
		}
	}

	return "", false
}

// ValidatePackSHA validates one pack version against a SHA checksum manifest.
func ValidatePackSHA(packVersion *config.PackVersion, manifestPath string) (*SHAValidationReport, error) {
	if packVersion == nil {
		return nil, fmt.Errorf("pack version is required")
	}

	packRoot := normalizePackRoot(packVersion.Path)
	if packRoot == "" {
		return nil, fmt.Errorf("pack path is empty")
	}

	resolvedManifest := strings.TrimSpace(manifestPath)
	if resolvedManifest == "" {
		var ok bool
		resolvedManifest, ok = ResolvePackSHAManifestPath(packVersion)
		if !ok || strings.TrimSpace(resolvedManifest) == "" {
			return nil, fmt.Errorf("no SHA manifest found for pack version %s", packVersion.Version)
		}
	}

	manifestData, err := readSHAData(resolvedManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to read SHA manifest %s: %w", resolvedManifest, err)
	}

	entries, err := parseSHAManifestEntries(manifestData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SHA manifest %s: %w", resolvedManifest, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("SHA manifest %s contains no checksum entries", resolvedManifest)
	}

	report := &SHAValidationReport{
		PackVersion:  packVersion.Version,
		ManifestPath: resolvedManifest,
		TotalEntries: len(entries),
	}

	for _, entry := range entries {
		if isSHAControlFile(entry.FilePath) {
			continue
		}

		data, readErr := readPackFileData(packRoot, entry.FilePath)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				report.MissingFiles = append(report.MissingFiles, entry.FilePath)
				continue
			}

			return nil, fmt.Errorf("failed to read pack file %s: %w", entry.FilePath, readErr)
		}

		hash := sha256.Sum256(data)
		actual := hex.EncodeToString(hash[:])
		if actual == entry.Expected {
			report.MatchedFiles++
			continue
		}

		report.Mismatches = append(report.Mismatches, SHAMismatch{
			FilePath: entry.FilePath,
			Expected: entry.Expected,
			Actual:   actual,
		})
	}

	sort.Strings(report.MissingFiles)
	sort.SliceStable(report.Mismatches, func(a, b int) bool {
		return report.Mismatches[a].FilePath < report.Mismatches[b].FilePath
	})

	return report, nil
}

type shaManifestEntry struct {
	Expected string
	FilePath string
}

func parseSHAManifestEntries(data []byte) ([]shaManifestEntry, error) {
	entries := make([]shaManifestEntry, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseSHALine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func parseSHALine(line string) (shaManifestEntry, error) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 2 {
		return shaManifestEntry{}, fmt.Errorf("invalid SHA line format")
	}

	expected := strings.ToLower(strings.TrimSpace(parts[0]))
	if !isSHA256Hex(expected) {
		return shaManifestEntry{}, fmt.Errorf("invalid SHA256 hash %q", parts[0])
	}

	filePath := strings.Join(parts[1:], " ")
	filePath = strings.TrimSpace(strings.Trim(filePath, "\""))
	filePath = strings.TrimPrefix(filePath, "*")
	filePath = cleanRelativeFilePath(filePath)
	if filePath == "" {
		return shaManifestEntry{}, fmt.Errorf("invalid file path in SHA line")
	}

	return shaManifestEntry{Expected: expected, FilePath: filePath}, nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}

	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}

	return true
}

func readSHAData(manifestPath string) ([]byte, error) {
	clean := strings.TrimSpace(manifestPath)
	if clean == "" {
		return nil, fmt.Errorf("manifest path is empty")
	}

	if filepath.IsAbs(clean) {
		return os.ReadFile(filepath.Clean(clean))
	}

	normalized := filepath.ToSlash(filepath.Clean(clean))
	normalized = strings.TrimPrefix(normalized, "./")

	if data, err := bundle.ReadFile(normalized); err == nil {
		return data, nil
	}

	return os.ReadFile(filepath.FromSlash(normalized))
}

func shaAssetExists(assetPath string) bool {
	clean := strings.TrimSpace(assetPath)
	if clean == "" {
		return false
	}

	if filepath.IsAbs(clean) {
		info, err := os.Stat(filepath.Clean(clean))
		return err == nil && !info.IsDir()
	}

	normalized := filepath.ToSlash(filepath.Clean(clean))
	normalized = strings.TrimPrefix(normalized, "./")

	if info, err := bundle.Stat(normalized); err == nil && !info.IsDir() {
		return true
	}

	if info, err := os.Stat(filepath.FromSlash(normalized)); err == nil && !info.IsDir() {
		return true
	}

	return false
}

func isSHAControlFile(filePath string) bool {
	clean := strings.ToLower(path.Base(filepath.ToSlash(strings.TrimSpace(filePath))))
	if clean == "" {
		return false
	}

	if strings.HasSuffix(clean, ".sha") || strings.HasSuffix(clean, ".sha256") || strings.HasSuffix(clean, ".sha512") {
		return true
	}

	if clean == "sha256.txt" || clean == "checksums.txt" || clean == "manifest.sha256" {
		return true
	}

	return false
}
