package safepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Inside resolves a nonempty package-relative path, rejecting traversal,
// alternate streams, absolute paths and pre-existing symlink ancestors.
func Inside(root, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.Contains(name, ":") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid relative path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("traversal path %q", name)
		}
		// Win32 strips trailing spaces/dots and treats device names specially, even with an extension.
		if part != "." && strings.TrimRight(part, " .") != part {
			return "", fmt.Errorf("ambiguous Windows path %q", name)
		}
		for _, char := range part {
			if char < 32 {
				return "", fmt.Errorf("control character in path %q", name)
			}
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		reserved := stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || stem == "CONIN$" || stem == "CONOUT$"
		if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
			reserved = true
		}
		if reserved {
			return "", fmt.Errorf("reserved Windows device path %q", name)
		}
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(base, filepath.FromSlash(name))
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %q", name)
	}
	// Lstat each ancestor; EvalSymlinks alone cannot handle a not-yet-created leaf.
	current := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink path %q", name)
		}
	}
	return target, nil
}

func PluginName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
