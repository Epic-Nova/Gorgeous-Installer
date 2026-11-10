package selfinstall

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type memoryPlatform struct {
	shortcut, registered, autostart bool
	fail                            bool
}

func (p *memoryPlatform) publish(Result, string) (func() error, error) {
	beforeShortcut, beforeRegistered := p.shortcut, p.registered
	undo := func() error { p.shortcut = beforeShortcut; p.registered = beforeRegistered; return nil }
	if p.fail {
		return undo, errors.New("registration failed")
	}
	p.shortcut = true
	p.registered = true
	return undo, nil
}
func (p *memoryPlatform) remove(Result) error { p.shortcut = false; p.registered = false; return nil }
func TestInstallPublishesLauncherAndUninstaller(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "portable executable.exe")
	if err := os.WriteFile(source, []byte("fixture binary"), 0755); err != nil {
		t.Fatal(err)
	}
	shell := &memoryPlatform{}
	result, err := install(Options{SourceExecutable: source, InstallDir: filepath.Join(root, "Programs", "Gorgeous Installer"), Version: "vTest"}, shell)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.Executable); err != nil {
		t.Fatal(err)
	}
	if !shell.shortcut || !shell.registered {
		t.Fatal("per-user installation did not publish both Start launcher and Installed Apps registration")
	}
	if shell.autostart {
		t.Fatal("installation enabled unrequested autostart")
	}
	if err := uninstall(result.Directory, shell); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.Executable); !os.IsNotExist(err) {
		t.Fatalf("installed executable remains: %v", err)
	}
	if shell.shortcut || shell.registered {
		t.Fatal("uninstall left shell integration behind")
	}
}
func TestRegistrationFailurePreservesPreviousBinary(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "portable.exe")
	directory := filepath.Join(root, "installed")
	if err := os.WriteFile(source, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	shell := &memoryPlatform{}
	result, err := install(Options{SourceExecutable: source, InstallDir: directory}, shell)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new binary"), 0755); err != nil {
		t.Fatal(err)
	}
	shell.fail = true
	if _, err = install(Options{SourceExecutable: source, InstallDir: directory}, shell); err == nil {
		t.Fatal("registration failure reported success")
	}
	data, err := os.ReadFile(result.Executable)
	if err != nil || string(data) != "old binary" {
		t.Fatal("previous executable was lost after failure")
	}
}
func TestUninstallRejectsUnownedDirectory(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "gorgeous-installer.exe")
	if err := os.WriteFile(path, []byte("user file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := uninstall(directory, &memoryPlatform{}); err == nil {
		t.Fatal("unowned directory was accepted")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("unowned file was removed")
	}
}
