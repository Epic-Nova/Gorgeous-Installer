package selfinstall

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Options struct{ SourceExecutable, InstallDir, Version string }
type Result struct{ Directory, Executable, Product string }
type platform interface {
	publish(Result, string) (func() error, error)
	remove(Result) error
}

const product = "EpicNova.GorgeousInstaller"
const manifestName = "gorgeous-installation.json"

// Install creates this user's Windows installation; no elevation is requested.
func Install(options Options) (Result, error) {
	shell, err := currentPlatform()
	if err != nil {
		return Result{}, err
	}
	if options.SourceExecutable == "" {
		options.SourceExecutable, err = os.Executable()
		if err != nil {
			return Result{}, err
		}
	}
	return install(options, shell)
}
func ExecutablePath(directory string) string {
	return filepath.Join(directory, "gorgeous-installer.exe")
}
func readInstallation(directory string) (Result, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil || directory == "" || filepath.Clean(directory) == "." || filepath.Dir(absolute) == absolute {
		return Result{}, errors.New("invalid installation directory")
	}
	data, err := os.ReadFile(filepath.Join(absolute, manifestName))
	if err != nil {
		return Result{}, fmt.Errorf("read installation record: %w", err)
	}
	var result Result
	if err = json.Unmarshal(data, &result); err != nil {
		return Result{}, err
	}
	if result.Product != product || !samePath(result.Directory, absolute) || !samePath(result.Executable, ExecutablePath(absolute)) {
		return Result{}, errors.New("directory is not a recorded Gorgeous Installer installation")
	}
	return result, nil
}
func IsInstalled(directory string) bool {
	result, err := readInstallation(directory)
	if err != nil {
		return false
	}
	info, err := os.Stat(result.Executable)
	return err == nil && info.Mode().IsRegular()
}
func install(options Options, shell platform) (Result, error) {
	if options.InstallDir == "" || !filepath.IsAbs(options.InstallDir) || filepath.Dir(filepath.Clean(options.InstallDir)) == filepath.Clean(options.InstallDir) {
		return Result{}, errors.New("choose an absolute installation folder below a drive root")
	}
	directory := filepath.Clean(options.InstallDir)
	result := Result{Directory: directory, Executable: ExecutablePath(directory), Product: product}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return result, err
	}
	source, err := os.Open(options.SourceExecutable)
	if err != nil {
		return result, err
	}
	defer source.Close()
	sourceInfo, err := source.Stat()
	if err != nil || !sourceInfo.Mode().IsRegular() {
		return result, errors.New("installation source is not a regular file")
	}
	destination, err := os.CreateTemp(directory, ".gorgeous-install-*")
	if err != nil {
		return result, err
	}
	temporary := destination.Name()
	defer os.Remove(temporary)
	_, copyErr := io.Copy(destination, source)
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return result, copyErr
	}
	if syncErr != nil {
		return result, syncErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	existing, statErr := os.Stat(result.Executable)
	same := statErr == nil && os.SameFile(sourceInfo, existing)
	backup := temporary + ".previous"
	if !same {
		if statErr == nil {
			if err := os.Rename(result.Executable, backup); err != nil {
				return result, fmt.Errorf("close the installed application before replacing it: %w", err)
			}
		}
		if err := os.Rename(temporary, result.Executable); err != nil {
			if statErr == nil {
				_ = os.Rename(backup, result.Executable)
			}
			return result, err
		}
	}
	written, statWrittenErr := os.Stat(result.Executable)
	rollback := func() error {
		if !same {
			current, currentErr := os.Stat(result.Executable)
			if statWrittenErr != nil || currentErr != nil || !os.SameFile(written, current) {
				return fmt.Errorf("installed executable changed before rollback; prior binary retained at %s", backup)
			}
			if err := os.Remove(result.Executable); err != nil {
				return err
			}
			if statErr == nil {
				return os.Rename(backup, result.Executable)
			}
		}
		return nil
	}
	undoShell, publishErr := shell.publish(result, options.Version)
	restoreShell := func() error {
		if undoShell != nil {
			return undoShell()
		}
		return nil
	}
	if publishErr != nil {
		return result, errors.Join(fmt.Errorf("register per-user application: %w", publishErr), restoreShell(), rollback())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	undoManifest, err := mutateOwnedFile(filepath.Join(directory, manifestName), func() error { return os.WriteFile(filepath.Join(directory, manifestName), data, 0644) })
	if err != nil {
		var manifestErr error
		if undoManifest != nil {
			manifestErr = undoManifest()
		}
		return result, errors.Join(err, manifestErr, restoreShell(), rollback())
	}
	_ = os.Remove(backup)
	return result, nil
}
func uninstall(directory string, shell platform) error {
	result, err := readInstallation(directory)
	if err != nil {
		return err
	}
	// Leave registration intact when another running instance holds the file.
	if err := os.Remove(result.Executable); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("close Gorgeous Installer before uninstalling: %w", err)
	}
	if err := shell.remove(result); err != nil {
		return err
	}
	for _, name := range []string{"uninstall-gorgeous.ps1", manifestName} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	_ = os.Remove(directory) // Nonempty folders, including user files, are preserved.
	return nil
}
