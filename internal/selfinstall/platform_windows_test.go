//go:build windows

package selfinstall

import (
	"context"
	"encoding/json"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWindowsNativeShellInstallAndUninstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profile space ' unicode ü")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "portable.exe")
	if err := os.WriteFile(source, []byte("native fixture executable"), 0755); err != nil {
		t.Fatal(err)
	}
	registryRoot := "Software\\EpicNova\\GorgeousInstallerValidation\\" + strconv.FormatInt(time.Now().UnixNano(), 16)
	settingsPath := filepath.Join(root, "Installer.json")
	if err := os.WriteFile(settingsPath, []byte("{\"uprojectAssociated\":true,\"prevUprojectCommand\":\"Previous.ProjectFile\",\"installedNatively\":true,\"allowNetwork\":false}"), 0644); err != nil {
		t.Fatal(err)
	}
	shell := windowsPlatform{filepath.Join(root, "Start Menu", "Programs"), registryRoot + "\\Uninstall", registryRoot + "\\Classes\\GorgeousInstaller.ProjectFile", settingsPath}
	t.Cleanup(func() {
		_ = shell.remove(Result{})
		_ = removeKey(registryRoot + "\\Classes\\.uproject")
		_ = removeKey(registryRoot + "\\Classes")
		_ = removeKey(registryRoot)
		_ = removeKey("Software\\EpicNova\\GorgeousInstallerValidation")
	})
	result, err := install(Options{SourceExecutable: source, InstallDir: filepath.Join(root, "Programs", "Gorgeous Installer"), Version: "vNativeFixture"}, shell)
	if err != nil {
		t.Fatal(err)
	}
	if !IsInstalled(result.Directory) {
		t.Fatal("installed record/executable not detected")
	}
	if _, err := runPowerShell("$s=New-Object -ComObject WScript.Shell; $l=$s.CreateShortcut(" + quotePS(shell.shortcut()) + "); if($l.TargetPath -ne " + quotePS(result.Executable) + " -or $l.Arguments -ne '--gui'){throw 'shortcut target/arguments do not match'}"); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"DisplayName": "Gorgeous Installer", "DisplayVersion": "vNativeFixture", "InstallLocation": result.Directory} {
		got, _, err := key.GetStringValue(name)
		if err != nil || got != want {
			t.Errorf("%s=%q: %v; want%q", name, got, err, want)
		}
	}
	command, _, err := key.GetStringValue("UninstallString")
	key.Close()
	if err != nil || !strings.Contains(command, quoteCommand(filepath.Join(result.Directory, "uninstall-gorgeous.ps1"))) {
		t.Fatalf("uninstall entry: %q %v", command, err)
	}
	if run, err := registry.OpenKey(registry.CURRENT_USER, registryRoot+"\\Run", registry.QUERY_VALUE); err == nil {
		run.Close()
		t.Fatal("unrequested autostart registration")
	}
	associationKey := registryRoot + "\\Classes\\.uproject"
	if existing, err := registry.OpenKey(registry.CURRENT_USER, associationKey, registry.QUERY_VALUE); err == nil {
		value, _, _ := existing.GetStringValue("")
		existing.Close()
		if value != "" {
			t.Fatal("installation changed the default file association without opt-in")
		}
	}
	openWith, err := registry.OpenKey(registry.CURRENT_USER, associationKey+"\\OpenWithProgids", registry.QUERY_VALUE)
	if err != nil {
		t.Fatal("Open With registration is absent")
	}
	_, _, err = openWith.GetStringValue("GorgeousInstaller.ProjectFile")
	openWith.Close()
	if err != nil {
		t.Fatal("Open With ProgID is absent")
	}
	association, _, err := registry.CreateKey(registry.CURRENT_USER, associationKey, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if err := association.SetStringValue("", "GorgeousInstaller.ProjectFile"); err != nil {
		t.Fatal(err)
	}
	association.Close()
	t.Cleanup(func() {
		_ = removeKey(associationKey)
		_ = removeKey(registryRoot + "\\Classes")
		_ = removeKey(registryRoot)
	})
	note := filepath.Join(result.Directory, "user-notes.txt")
	if err := os.WriteFile(note, []byte("preserve"), 0644); err != nil {
		t.Fatal(err)
	}
	moved, err := install(Options{SourceExecutable: source, InstallDir: filepath.Join(root, "Relocated Programs", "Gorgeous Installer"), Version: "vMoved"}, shell)
	if err != nil {
		t.Fatal(err)
	}
	runUninstaller := func(directory string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, powershell(), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(directory, "uninstall-gorgeous.ps1"), "-Quiet")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("actual registered uninstaller failed: %v: %s", err, output)
		}
	}
	runUninstaller(result.Directory)
	if _, err := os.Stat(result.Executable); !os.IsNotExist(err) {
		t.Fatalf("exe was not removed: %v", err)
	}
	key, err = registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("old install removed the moved install's registration: %v", err)
	}
	location, _, _ := key.GetStringValue("InstallLocation")
	key.Close()
	if location != moved.Directory {
		t.Fatal("moved installation registration changed")
	}
	if _, err := runPowerShell("$s=New-Object -ComObject WScript.Shell; if($s.CreateShortcut(" + quotePS(shell.shortcut()) + ").TargetPath -ne " + quotePS(moved.Executable) + "){throw 'moved shortcut was removed or changed'}"); err != nil {
		t.Fatal(err)
	}
	runUninstaller(moved.Directory)
	if _, err := os.Stat(shell.shortcut()); !os.IsNotExist(err) {
		t.Fatalf("Start shortcut was not removed: %v", err)
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE); err == nil {
		key.Close()
		t.Fatal("Installed Apps registration remains")
	}
	if data, err := os.ReadFile(note); err != nil || string(data) != "preserve" {
		t.Fatal("uninstaller removed unrelated user data")
	}
	association, err = registry.OpenKey(registry.CURRENT_USER, associationKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, _ := association.GetStringValue("")
	association.Close()
	if restored != "Previous.ProjectFile" {
		t.Fatalf("previous file association was not restored: %q", restored)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var preferences map[string]any
	if err := json.Unmarshal(data, &preferences); err != nil {
		t.Fatal(err)
	}
	if preferences["uprojectAssociated"] != false || preferences["installedNatively"] != false || preferences["allowNetwork"] != false {
		t.Fatal("preferences were not preserved/reset correctly")
	}
	t.Log("Native fixture: COM shortcut target/arguments, HKCU Installed Apps fields, actual uninstall script and unrelated-file preservation verified; no real user Start folder or production registration modified.")
}

func TestWindowsDeferredSelfUninstall(t *testing.T) {
	if os.Getenv("GORGEOUS_SELF_UNINSTALL_TEST_CHILD") == "1" {
		deferred, err := Uninstall(os.Getenv("GORGEOUS_SELF_UNINSTALL_TEST_DIRECTORY"))
		if err != nil || !deferred {
			t.Fatalf("running installed process did not schedule its own uninstall: %v", err)
		}
		return
	}
	root := filepath.Join(t.TempDir(), "self uninstall space ü")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	registryRoot := "Software\\EpicNova\\GorgeousInstallerValidation\\" + strconv.FormatInt(time.Now().UnixNano(), 16)
	preferences := filepath.Join(root, "Installer.json")
	if err := os.WriteFile(preferences, []byte("{\"allowNetwork\":false}"), 0644); err != nil {
		t.Fatal(err)
	}
	shell := windowsPlatform{filepath.Join(root, "Start Menu"), registryRoot + "\\Uninstall", registryRoot + "\\Classes\\GorgeousInstaller.ProjectFile", preferences}
	t.Cleanup(func() {
		_ = shell.remove(Result{})
		_ = removeKey(registryRoot + "\\Classes\\.uproject")
		_ = removeKey(registryRoot + "\\Classes")
		_ = removeKey(registryRoot)
		_ = removeKey("Software\\EpicNova\\GorgeousInstallerValidation")
	})
	result, err := install(Options{SourceExecutable: source, InstallDir: filepath.Join(root, "Installed"), Version: "vChild"}, shell)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, result.Executable, "-test.run=^TestWindowsDeferredSelfUninstall$", "-test.v")
	child.Env = append(os.Environ(), "GORGEOUS_SELF_UNINSTALL_TEST_CHILD=1", "GORGEOUS_SELF_UNINSTALL_TEST_DIRECTORY="+result.Directory)
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("installed native child failed: %v: %s", err, output)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, exeErr := os.Stat(result.Executable)
		_, linkErr := os.Stat(shell.shortcut())
		key, regErr := registry.OpenKey(registry.CURRENT_USER, shell.uninstall, registry.QUERY_VALUE)
		if regErr == nil {
			key.Close()
		}
		if os.IsNotExist(exeErr) && os.IsNotExist(linkErr) && regErr == registry.ErrNotExist {
			t.Log("Real copied native process scheduled hidden PID-aware uninstall, exited naturally, and its helper removed executable, shortcut and registry entry.")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("deferred uninstall did not finish after the installed process exited: %s", output)
}
