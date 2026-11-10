//go:build windows

package selfinstall

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

type failingWindowsShell struct {
	windowsPlatform
	stage         string
	beforeFailure func() error
}

func (p failingWindowsShell) publish(result Result, version string) (func() error, error) {
	return p.windowsPlatform.publishWithFault(result, version, func(stage string) error {
		if stage != p.stage {
			return nil
		}
		if p.beforeFailure != nil {
			if err := p.beforeFailure(); err != nil {
				return err
			}
		}
		return errors.New("injected late shell registration failure")
	})
}
func nativeRollbackFixture(t *testing.T) (Options, windowsPlatform, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "portable.exe")
	if err := os.WriteFile(source, []byte("old executable"), 0755); err != nil {
		t.Fatal(err)
	}
	registryRoot := "Software\\EpicNova\\GorgeousInstallerRollbackTests\\" + strconv.FormatInt(time.Now().UnixNano(), 16)
	shell := windowsPlatform{filepath.Join(root, "Start Menu"), registryRoot + "\\Uninstall", registryRoot + "\\Classes\\GorgeousInstaller.ProjectFile", filepath.Join(root, "Installer.json")}
	t.Cleanup(func() {
		_ = shell.remove(Result{})
		for _, path := range []string{registryRoot + "\\Classes\\.uproject", registryRoot + "\\Classes", registryRoot, "Software\\EpicNova\\GorgeousInstallerRollbackTests"} {
			_ = removeKey(path)
		}
	})
	return Options{SourceExecutable: source, InstallDir: filepath.Join(root, "Installed"), Version: "vOld"}, shell, root
}

type registryTarget struct{ path, name string }

func shellValueSnapshot(t *testing.T, p windowsPlatform) map[registryTarget]registryValueState {
	t.Helper()
	result := map[registryTarget]registryValueState{}
	var targets []registryTarget
	for _, name := range []string{"DisplayName", "DisplayVersion", "Publisher", "InstallLocation", "DisplayIcon", "UninstallString", "QuietUninstallString", "NoModify", "NoRepair"} {
		targets = append(targets, registryTarget{p.uninstall, name})
	}
	targets = append(targets, registryTarget{p.project, ""}, registryTarget{p.project + "\\shell\\open\\command", ""}, registryTarget{p.project[:strings.LastIndex(p.project, "\\")] + "\\.uproject\\OpenWithProgids", "GorgeousInstaller.ProjectFile"})
	for _, target := range targets {
		state, err := readRegistryValue(target.path, target.name)
		if err != nil {
			t.Fatal(err)
		}
		result[target] = state
	}
	return result
}
func assertShellValues(t *testing.T, want map[registryTarget]registryValueState) {
	t.Helper()
	for target, before := range want {
		after, err := readRegistryValue(target.path, target.name)
		if err != nil || !before.equal(after) {
			t.Errorf("registry value not restored: %s/%s: %v", target.path, target.name, err)
		}
	}
}
func TestWindowsNewInstallRollsBackEveryPublicationStage(t *testing.T) {
	for _, stage := range []string{"script", "shortcut", "uninstall", "complete"} {
		t.Run(stage, func(t *testing.T) {
			options, shell, _ := nativeRollbackFixture(t)
			before := shellValueSnapshot(t, shell)
			result, err := install(options, failingWindowsShell{shell, stage, nil})
			if err == nil {
				t.Fatal("failure was not injected")
			}
			for _, path := range []string{result.Executable, shell.shortcut(), filepath.Join(options.InstallDir, "uninstall-gorgeous.ps1"), filepath.Join(options.InstallDir, manifestName)} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("new publication file remains: %s: %v", path, err)
				}
			}
			assertShellValues(t, before)
		})
	}
}
func TestWindowsRepairRelocationAndManifestFailureRestorePreviousShell(t *testing.T) {
	for _, mode := range []string{"repair", "relocation", "manifest"} {
		t.Run(mode, func(t *testing.T) {
			options, shell, root := nativeRollbackFixture(t)
			previous, err := install(options, shell)
			if err != nil {
				t.Fatal(err)
			}
			openWithPath := shell.project[:strings.LastIndex(shell.project, "\\")] + "\\.uproject\\OpenWithProgids"
			key, err := registry.OpenKey(registry.CURRENT_USER, openWithPath, registry.SET_VALUE)
			if err != nil {
				t.Fatal(err)
			}
			if err := restoreRegistryValue(key, "GorgeousInstaller.ProjectFile", registryValueState{true, registry.NONE, nil}); err != nil {
				t.Fatal(err)
			}
			key.Close()
			before := shellValueSnapshot(t, shell)
			shortcut, err := os.ReadFile(shell.shortcut())
			if err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(previous.Directory, "uninstall-gorgeous.ps1"))
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := os.ReadFile(filepath.Join(previous.Directory, manifestName))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(options.SourceExecutable, []byte("new executable"), 0755); err != nil {
				t.Fatal(err)
			}
			options.Version = "vNew"
			var selected platform = failingWindowsShell{shell, "complete", nil}
			if mode != "repair" {
				options.InstallDir = filepath.Join(root, "Relocated")
				if err := os.MkdirAll(options.InstallDir, 0755); err != nil {
					t.Fatal(err)
				}
				if mode == "relocation" {
					if err := os.WriteFile(filepath.Join(options.InstallDir, "uninstall-gorgeous.ps1"), []byte("preexisting user file"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "manifest" {
				if err := os.Mkdir(filepath.Join(options.InstallDir, manifestName), 0755); err != nil {
					t.Fatal(err)
				}
				selected = shell
			}
			if _, err := install(options, selected); err == nil {
				t.Fatal("late failure was not reached")
			}
			assertShellValues(t, before)
			if data, err := os.ReadFile(previous.Executable); err != nil || string(data) != "old executable" {
				t.Fatal("previous executable was not restored")
			}
			if data, err := os.ReadFile(shell.shortcut()); err != nil || !bytes.Equal(data, shortcut) {
				t.Fatal("previous Start shortcut was not restored byte-for-byte")
			}
			if data, err := os.ReadFile(filepath.Join(previous.Directory, "uninstall-gorgeous.ps1")); err != nil || !bytes.Equal(data, script) {
				t.Fatal("previous uninstaller was not restored")
			}
			if data, err := os.ReadFile(filepath.Join(previous.Directory, manifestName)); err != nil || !bytes.Equal(data, manifest) {
				t.Fatal("previous manifest changed")
			}
			if mode == "relocation" {
				if data, err := os.ReadFile(filepath.Join(options.InstallDir, "uninstall-gorgeous.ps1")); err != nil || string(data) != "preexisting user file" {
					t.Fatal("preexisting destination file was lost")
				}
			}
			if mode == "manifest" {
				if info, err := os.Stat(filepath.Join(options.InstallDir, manifestName)); err != nil || !info.IsDir() {
					t.Fatal("preexisting manifest-name directory was changed")
				}
			}
		})
	}
}
func TestWindowsRollbackPreservesNewerInstallation(t *testing.T) {
	options, shell, root := nativeRollbackFixture(t)
	if _, err := install(options, shell); err != nil {
		t.Fatal(err)
	}
	options.InstallDir = filepath.Join(root, "Failed relocation")
	options.Version = "vFailed"
	var newer Result
	failing := failingWindowsShell{shell, "complete", func() error {
		var err error
		newer, err = install(Options{SourceExecutable: options.SourceExecutable, InstallDir: filepath.Join(root, "Newer user installation"), Version: "vUser"}, shell)
		return err
	}}
	if _, err := install(options, failing); err == nil {
		t.Fatal("failure was not injected")
	}
	if !IsInstalled(newer.Directory) {
		t.Fatal("newer installation was removed")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	location, _, _ := key.GetStringValue("InstallLocation")
	version, _, _ := key.GetStringValue("DisplayVersion")
	key.Close()
	if location != newer.Directory || version != "vUser" {
		t.Fatal("newer Installed Apps choice was overwritten")
	}
	if _, err := runPowerShell("$s=New-Object -ComObject WScript.Shell; if($s.CreateShortcut(" + quotePS(shell.shortcut()) + ").TargetPath -ne " + quotePS(newer.Executable) + "){throw 'newer Start choice was overwritten'}"); err != nil {
		t.Fatal(err)
	}
	openWith, err := readRegistryValue(shell.project[:strings.LastIndex(shell.project, "\\")]+"\\.uproject\\OpenWithProgids", "GorgeousInstaller.ProjectFile")
	if err != nil || !openWith.exists {
		t.Fatal("newer Open With registration was removed")
	}
	if _, err := os.Stat(ExecutablePath(options.InstallDir)); !os.IsNotExist(err) {
		t.Fatal("failed relocation executable remains")
	}
}
func TestWindowsRollbackRestoresOwnedValuesAndKeepsUnrelatedEdits(t *testing.T) {
	options, shell, _ := nativeRollbackFixture(t)
	if _, err := install(options, shell); err != nil {
		t.Fatal(err)
	}
	before := shellValueSnapshot(t, shell)
	options.Version = "vFailed"
	failing := failingWindowsShell{shell, "complete", func() error {
		key, err := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()
		return key.SetStringValue("UnrelatedUserValue", "keep newer user edit")
	}}
	if _, err := install(options, failing); err == nil {
		t.Fatal("failure was not injected")
	}
	assertShellValues(t, before)
	key, err := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue("UnrelatedUserValue")
	if err != nil || value != "keep newer user edit" {
		t.Fatal("unrelated user edit was overwritten")
	}
}
