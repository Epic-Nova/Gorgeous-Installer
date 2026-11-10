package ui

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func patchPublisherWinresVersion(data []byte, version string) ([]byte, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	match := regexp.MustCompile(`^(?:Dev[.-])?([0-9]+)\.([0-9]+)\.([0-9]+)(?:\.([0-9]+))?(?:[-+][0-9A-Za-z][0-9A-Za-z.-]*)?$`).FindStringSubmatch(version)
	if match == nil {
		return nil, fmt.Errorf("invalid Windows release version: %q", version)
	}
	parts := append([]string(nil), match[1:5]...)
	if parts[3] == "" {
		parts[3] = "0"
	}
	for index, part := range parts {
		value, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("Windows version component must be between 0 and 65535: %q", part)
		}
		parts[index] = strconv.FormatUint(value, 10)
	}
	fourPart := strings.Join(parts, ".")
	var resource map[string]any
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &resource); err != nil {
		return nil, err
	}
	versions, ok := resource["RT_VERSION"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("winres.json has no RT_VERSION object")
	}
	changed := 0
	for _, rawIDs := range versions {
		ids, ok := rawIDs.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Windows resource version identifier")
		}
		for _, rawEntry := range ids {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid Windows version resource")
			}
			fixed, ok := entry["fixed"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("Windows version resource has no fixed fields")
			}
			info, ok := entry["info"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("Windows version resource has no text fields")
			}
			fixed["file_version"], fixed["product_version"] = fourPart, fourPart
			for _, rawText := range info {
				text, ok := rawText.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid localized Windows version fields")
				}
				text["FileVersion"], text["ProductVersion"] = fourPart, version
			}
			changed++
		}
	}
	if changed == 0 {
		return nil, fmt.Errorf("winres.json contains no version resource entries")
	}
	patched, err := json.MarshalIndent(resource, "", "  ")
	return append(patched, '\n'), err
}

func publisherBuildCommand(sourceDir, goos string, clean, crossWindows bool) *exec.Cmd {
	if goos == "windows" {
		args := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(sourceDir, "build.ps1")}
		if clean {
			args = append(args, "-Clean")
		}
		cmd := exec.Command("powershell.exe", args...)
		cmd.Dir = sourceDir
		// PowerShell7 parents can pass a module path which omits Windows
		// PowerShell's stock modules (including Get-FileHash). Retain user paths.
		if filepath.IsAbs(cmd.Path) {
			modules := filepath.Join(filepath.Dir(cmd.Path), "Modules")
			cmd.Env = append(os.Environ(), "PSModulePath="+modules+string(os.PathListSeparator)+os.Getenv("PSModulePath"))
		}
		return cmd
	}
	args := []string{filepath.Join(sourceDir, "build.sh")}
	if clean {
		args = append(args, "--clean")
	}
	if crossWindows {
		args = append(args, "--cross-windows")
	}
	cmd := exec.Command("bash", args...)
	cmd.Dir = sourceDir
	return cmd
}

func runPublisherBuild(cmd *exec.Cmd, log func(string, ...any)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return err
	}
	reader := bufio.NewReader(stdout)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			log("%s", strings.TrimRight(line, "\r\n"))
		}
		if readErr != nil {
			if readErr != io.EOF {
				_ = cmd.Wait()
				return readErr
			}
			break
		}
	}
	return cmd.Wait()
}

func writePublisherZIP(sourceDir, destination string, installerSource bool) error {
	source, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("archive source is not a directory: %s", source)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(output), ".publisher-*.zip")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	temporaryInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	existingOutput, err := os.Stat(output)
	if err != nil && !os.IsNotExist(err) {
		_ = file.Close()
		return err
	}
	archive := zip.NewWriter(file)
	err = walkPublisherTree(source, func(path, rel string, info os.FileInfo) error {
		if publisherSamePath(path, source) || publisherSamePath(path, output) || publisherSamePath(path, temporary) {
			return nil
		}
		if os.SameFile(info, temporaryInfo) || (existingOutput != nil && os.SameFile(info, existingOutput)) {
			return nil
		}
		name := filepath.ToSlash(rel)
		lower := strings.ToLower(name)
		if installerSource && (lower == "build" || strings.HasPrefix(lower, "build/") || lower == ".git" || strings.HasPrefix(lower, ".git/") || strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".syso") || strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".gti")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported archive entry: %s", path)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil || info.IsDir() {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	zipErr := archive.Close()
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if zipErr != nil {
		return zipErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temporary, output)
}

func copyPublisherTree(sourceDir, destination string) error {
	source, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("copy source is not a directory: %s", source)
	}
	targetInfo, err := os.Stat(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if publisherSamePath(source, target) || (targetInfo != nil && os.SameFile(sourceInfo, targetInfo)) {
		return nil
	}
	rel, err := filepath.Rel(source, target)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("copy destination is inside source: %s", target)
	}
	return walkPublisherTree(source, func(path, rel string, info os.FileInfo) error {
		if info.IsDir() {
			if rel != "." && targetInfo != nil && os.SameFile(info, targetInfo) {
				return fmt.Errorf("copy destination is reached through a directory link: %s", path)
			}
			if err := os.MkdirAll(filepath.Join(target, rel), info.Mode().Perm()|0700); err != nil {
				return err
			}
			if rel == "." {
				targetInfo, err = os.Stat(target)
				return err
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported copy entry: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(filepath.Join(target, rel), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return closeErr
	})
}

// Like the original zip tool, publish linked directory contents as normal
// portable payload files. Detect link cycles before descending indefinitely.
func walkPublisherTree(root string, visit func(string, string, os.FileInfo) error) error {
	var walk func(string, string, []os.FileInfo) error
	walk = func(path, rel string, ancestors []os.FileInfo) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := visit(path, rel, info); err != nil {
			if err == filepath.SkipDir {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return nil
		}
		for _, ancestor := range ancestors {
			if os.SameFile(info, ancestor) {
				return fmt.Errorf("directory link cycle in publisher input: %s", path)
			}
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := walk(filepath.Join(path, entry.Name()), filepath.Join(rel, entry.Name()), append(ancestors, info)); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root, ".", nil)
}

func publisherSamePath(a, b string) bool {
	// filepath.Rel applies native volume/case rules, including Windows drives.
	rel, err := filepath.Rel(a, b)
	return err == nil && rel == "."
}
