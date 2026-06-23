package installer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/unreal"
)

type UpdateInstruction struct {
	Action string `json:"Action"` // Repair, Delete, Update, Install
	Target string `json:"Target"` // Relative path
	Source string `json:"Source"` // Relative path in zip
}

type UpdateManifest struct {
	PluginName   string              `json:"PluginName"`
	Version      string              `json:"Version"`
	Instructions []UpdateInstruction `json:"Instructions"`
}

func ProcessZipUpdate(zipPath, projectPath string, pidToWait int) error {
	if pidToWait > 0 {
		fmt.Printf("Waiting for Unreal Editor (PID: %d) to close gracefully...\n", pidToWait)
		for {
			if err := checkProcessAlive(pidToWait); err != nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	tempDir, err := os.MkdirTemp("", "gorgeous-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := extractZip(zipPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract zip: %w", err)
	}

	manifestPath := filepath.Join(tempDir, "UpdateInstructions.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("UpdateInstructions.json not found in zip")
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read UpdateInstructions.json: %w", err)
	}

	var manifest UpdateManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse UpdateInstructions.json: %w", err)
	}

	// Resolve the plugin directory
	version, enginePath, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		return fmt.Errorf("failed to locate project/engine: %w", err)
	}
	_ = version

	pluginPath, err := unreal.LocatePluginByName(projectPath, enginePath, manifest.PluginName)
	if err != nil {
		return fmt.Errorf("failed to locate plugin %s: %w", manifest.PluginName, err)
	}

	fmt.Printf("Applying updates to %s at %s\n", manifest.PluginName, pluginPath)

	for _, inst := range manifest.Instructions {
		targetAbs := filepath.Join(pluginPath, filepath.FromSlash(inst.Target))
		sourceAbs := filepath.Join(tempDir, filepath.FromSlash(inst.Source))

		switch strings.ToUpper(inst.Action) {
		case "DELETE":
			fmt.Printf("Deleting %s\n", targetAbs)
			os.RemoveAll(targetAbs)
		case "UPDATE", "REPAIR", "INSTALL":
			fmt.Printf("%s %s\n", inst.Action, targetAbs)
			if err := copyFileOrDir(sourceAbs, targetAbs); err != nil {
				return fmt.Errorf("failed to %s %s: %w", inst.Action, inst.Target, err)
			}
		default:
			fmt.Printf("Unknown action: %s\n", inst.Action)
		}
	}

	fmt.Println("Updates applied successfully! Initiating recompilation...")

	// Launch Recompile — pass a zero-value PackVersion so NewInstaller gets a valid pointer.
	dummyVersion := &config.PackVersion{}
	inst := NewInstaller(pluginPath, "code", dummyVersion, pluginPath, projectPath, enginePath)
	inst.RecompileOnly = true
	inst.StatusLogger = func(msg string, args ...any) {
		fmt.Printf("[Compile] "+msg+"\n", args...)
	}
	return inst.Install()
}

func checkProcessAlive(pid int) error {
	for {
		if !isProcessRunning(pid) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

func copyFileOrDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return zipCopyFile(src, dst)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, os.ModePerm)
		}
		return zipCopyFile(path, target)
	})
}

func zipCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	os.MkdirAll(filepath.Dir(dst), os.ModePerm)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
