package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	bundle "gorgeous-installer"
)

// InstallAction controls whether files are copied as a full install, diff update, or forced reinstall.
type InstallAction string

const (
	InstallActionInstall   InstallAction = "install"
	InstallActionUpdate    InstallAction = "update"
	InstallActionReinstall InstallAction = "reinstall"
)

// InstallPlan describes what would be copied for a selected pack version.
type InstallPlan struct {
	Action            InstallAction
	PackType          string
	PackVersion       string
	SourceRoot        string
	DestinationRoot   string
	TotalSourceFiles  int
	ExistingFiles     int
	UnchangedFiles    int
	MissingFiles      []string
	DifferentFiles    []string
	ChangedFiles      []string
	DestinationExists bool
}

// BuildInstallPlan compares pack files against destination files and returns the action to run.
func (i *Installer) BuildInstallPlan() (*InstallPlan, error) {
	if i.PackVersion == nil {
		return nil, fmt.Errorf("pack version is required")
	}

	sourceRoot := normalizePackRoot(i.PackVersion.Path)
	if sourceRoot == "" {
		return nil, fmt.Errorf("pack path is empty")
	}

	destinationRoot, err := i.resolveDestinationRootForPackType()
	if err != nil {
		return nil, err
	}

	if info, statErr := os.Stat(destinationRoot); statErr == nil && info.IsDir() {
		_ = info
	} else if statErr == nil {
		return nil, fmt.Errorf("install destination is not a directory: %s", destinationRoot)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to inspect install destination %s: %w", destinationRoot, statErr)
	}

	sourceFiles, err := listPackFiles(sourceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to list pack files from %s: %w", sourceRoot, err)
	}
	if len(sourceFiles) == 0 {
		return nil, fmt.Errorf("pack path contains no files: %s", sourceRoot)
	}

	if err := validateExampleFiles(i.PackType, sourceFiles); err != nil {
		return nil, err
	}
	plan := &InstallPlan{
		PackType:         i.PackType,
		PackVersion:      i.PackVersion.Version,
		SourceRoot:       sourceRoot,
		DestinationRoot:  destinationRoot,
		TotalSourceFiles: len(sourceFiles),
	}

	if info, err := os.Stat(destinationRoot); err == nil && info.IsDir() {
		plan.DestinationExists = true
	}

	for _, rel := range sourceFiles {
		srcData, err := readPackFileData(sourceRoot, rel)
		if err != nil {
			return nil, fmt.Errorf("failed to read source pack file %s: %w", rel, err)
		}

		dstPath := filepath.Join(destinationRoot, filepath.FromSlash(rel))
		dstData, err := os.ReadFile(dstPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				plan.MissingFiles = append(plan.MissingFiles, rel)
				plan.ChangedFiles = append(plan.ChangedFiles, rel)
				continue
			}

			return nil, fmt.Errorf("failed to read destination file %s: %w", dstPath, err)
		}

		plan.ExistingFiles++
		srcHash := hashBytesSHA256(srcData)
		dstHash := hashBytesSHA256(dstData)
		if srcHash == dstHash {
			plan.UnchangedFiles++
			continue
		}

		plan.DifferentFiles = append(plan.DifferentFiles, rel)
		plan.ChangedFiles = append(plan.ChangedFiles, rel)
	}

	sort.Strings(plan.MissingFiles)
	sort.Strings(plan.DifferentFiles)
	sort.Strings(plan.ChangedFiles)

	switch {
	case len(plan.ChangedFiles) > 0 && plan.ExistingFiles > 0:
		plan.Action = InstallActionUpdate
	case len(plan.ChangedFiles) > 0:
		plan.Action = InstallActionInstall
	case plan.ExistingFiles > 0:
		plan.Action = InstallActionReinstall
	default:
		plan.Action = InstallActionInstall
	}

	return plan, nil
}

func (i *Installer) resolveDestinationRootForPackType() (string, error) {
	switch strings.ToLower(strings.TrimSpace(i.PackType)) {
	case "game-feature-example":
  projectFile, err := resolveProjectFilePath(i.ProjectPath)
  if err != nil { return "",err }
  files, err := listPackFiles(normalizePackRoot(i.PackVersion.Path))
  if err != nil { return "",err }
  name,err := validateGameFeaturePayload(normalizePackRoot(i.PackVersion.Path),files)
  if err != nil {return "",err}
  return filepath.Join(filepath.Dir(projectFile),"Plugins","GameFeatures",name),nil
 case "content-example":
		projectFile, err := resolveProjectFilePath(i.ProjectPath)
		if err != nil {
			return "", err
		}
		return i.resolveInstallDir(filepath.Join(filepath.Dir(projectFile), "Content"), "content")
	case "project-example":
		target, err := filepath.Abs(i.ProjectPath)
		if err != nil || strings.TrimSpace(i.ProjectPath) == "" {
			return "", fmt.Errorf("example destination directory is required")
		}
		if strings.EqualFold(filepath.Ext(target), ".uproject") {
			return "", fmt.Errorf("full project examples require a destination directory, not an existing .uproject")
		}
		if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
			return "", fmt.Errorf("full project examples require an empty destination directory")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return target, nil
	case "content":
		contentRoot := filepath.Join(i.PluginPath, "Content")
		return i.resolveInstallDir(contentRoot, "content")
	case "code":
		sourceRoot := filepath.Join(i.PluginPath, "Source")
		return i.resolveInstallDir(sourceRoot, "source")
	case "hybrid":
		return i.PluginPath, nil
	default:
		return "", fmt.Errorf("unknown pack type: %s", i.PackType)
	}
}

func normalizePackRoot(packPath string) string {
	clean := strings.TrimSpace(packPath)
	if clean == "" {
		return ""
	}

	clean = filepath.ToSlash(filepath.Clean(clean))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." {
		return ""
	}

	return clean
}

func listPackFiles(sourceRoot string) ([]string, error) {
	normalized := normalizePackRoot(sourceRoot)
	if normalized == "" {
		return nil, fmt.Errorf("source root is empty")
	}

	embeddedFiles, embeddedErr := listEmbeddedPackFiles(normalized)
	if embeddedErr == nil && len(embeddedFiles) > 0 {
		sort.Strings(embeddedFiles)
		return embeddedFiles, nil
	}

	diskFiles, diskErr := listDiskPackFiles(filepath.FromSlash(normalized))
	if diskErr == nil && len(diskFiles) > 0 {
		sort.Strings(diskFiles)
		return diskFiles, nil
	}

	if embeddedErr != nil {
		if diskErr != nil {
			return nil, fmt.Errorf("embedded read failed: %v; disk read failed: %v", embeddedErr, diskErr)
		}
		return nil, embeddedErr
	}

	if diskErr != nil {
		return nil, diskErr
	}

	return nil, fmt.Errorf("no files found in pack source %s", normalized)
}

func listEmbeddedPackFiles(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("embedded root is empty")
	}

	files := make([]string, 0)
	if err := walkEmbeddedPackFiles(root, root, &files); err != nil {
		return nil, err
	}

	return files, nil
}

func walkEmbeddedPackFiles(root, current string, files *[]string) error {
	entries, err := bundle.ReadDir(current)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		child := path.Join(current, entry.Name())
		if entry.IsDir() {
			if err := walkEmbeddedPackFiles(root, child, files); err != nil {
				return err
			}
			continue
		}

		rel := strings.TrimPrefix(child, root+"/")
		if rel == child {
			rel = entry.Name()
		}
		if rel == "." || rel == "" {
			continue
		}
		if isSHAControlFile(rel) {
			continue
		}

		*files = append(*files, rel)
	}

	return nil
}

func hashBytesSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func listDiskPackFiles(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("disk root is empty")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("pack source must be a directory: %s", root)
	}

	files := make([]string, 0)
	walkErr := filepath.WalkDir(root, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(root, filePath)
		if relErr != nil {
			return relErr
		}

		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		if isSHAControlFile(rel) {
			return nil
		}

		files = append(files, rel)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return files, nil
}

func readPackFileData(sourceRoot, rel string) ([]byte, error) {
	cleanRoot := normalizePackRoot(sourceRoot)
	cleanRel := cleanRelativeFilePath(rel)
	if cleanRoot == "" || cleanRel == "" {
		return nil, fmt.Errorf("invalid source root or relative file path")
	}

	embeddedPath := path.Join(cleanRoot, cleanRel)
	if data, err := bundle.ReadFile(embeddedPath); err == nil {
		return data, nil
	}

	diskPath := filepath.Join(filepath.FromSlash(cleanRoot), filepath.FromSlash(cleanRel))
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file %s: %w", diskPath, err)
	}

	return data, nil
}

func cleanRelativeFilePath(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}

	return clean
}

func copyPackFiles(sourceRoot, destinationRoot string, relFiles []string) error {
	if len(relFiles) == 0 {
		return nil
	}

	cleanRoot := normalizePackRoot(sourceRoot)
	if cleanRoot == "" {
		return fmt.Errorf("source root is empty")
	}
	if strings.TrimSpace(destinationRoot) == "" {
		return fmt.Errorf("destination root is empty")
	}

	for _, rel := range relFiles {
		if isSHAControlFile(rel) {
			continue
		}

		cleanRel := cleanRelativeFilePath(rel)
		if cleanRel == "" {
			return fmt.Errorf("invalid relative path in file list: %q", rel)
		}

		data, err := readPackFileData(cleanRoot, cleanRel)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(destinationRoot, filepath.FromSlash(cleanRel))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory for %s: %w", dstPath, err)
		}

		mode := os.FileMode(0644)
		diskSrcPath := filepath.Join(filepath.FromSlash(cleanRoot), filepath.FromSlash(cleanRel))
		if info, err := os.Stat(diskSrcPath); err == nil && !info.IsDir() {
			mode = info.Mode()
		}

		if err := os.WriteFile(dstPath, data, mode); err != nil {
			return fmt.Errorf("failed to write destination file %s: %w", dstPath, err)
		}
	}

	return nil
}

// ValidateExamplePayload checks example layout before packaging or installation.
func ValidateExamplePayload(packType, root string) error {
	files, err := listPackFiles(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("example payload contains no files")
	}
	return validateExampleFiles(packType, files)
}

func validateExampleFiles(packType string, files []string) error {
	if packType == "game-feature-example" { _,err:=validateGameFeaturePayload(root,files);return err }
 projectFiles := 0
	for _, rel := range files {
		first := strings.Split(filepath.ToSlash(rel), "/")[0]
		if packType == "content-example" {
			if strings.EqualFold(first, "Content") || strings.EqualFold(first, "Config") || strings.EqualFold(first, "Plugins") || strings.EqualFold(first, "Source") || strings.EqualFold(filepath.Ext(rel), ".uproject") {
				return fmt.Errorf("content-example payload must contain the contents of Content, without a Content wrapper or project files: %s", rel)
			}
		}
		if packType == "project-example" && !strings.Contains(filepath.ToSlash(rel), "/") && strings.EqualFold(filepath.Ext(rel), ".uproject") {
			projectFiles++
		}
	}
	if packType == "project-example" && projectFiles != 1 {
		return fmt.Errorf("project-example payload must contain exactly one root .uproject file")
	}
	return nil
}
