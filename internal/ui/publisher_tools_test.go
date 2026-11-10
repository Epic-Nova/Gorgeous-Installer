package ui

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublisherPrereleaseKeepsNumericWindowsResources(t *testing.T) {
	data := []byte(`{"RT_GROUP_ICON":{"APP":{"0000":["icon.png"]}},"RT_VERSION":{"#1":{"0000":{"fixed":{"product_version":"1.0.0.0","file_version":"1.0.0.0"},"info":{"0409":{"ProductVersion":"1.0.0.0","FileVersion":"1.0.0.0","ProductName":"Installer"}}}}}}`)
	patched, err := patchPublisherWinresVersion(data, "2.7.9-Dev.4")
	if err != nil {
		t.Fatal(err)
	}
	var resource struct {
		Versions map[string]map[string]struct {
			Fixed map[string]string            `json:"fixed"`
			Info  map[string]map[string]string `json:"info"`
		} `json:"RT_VERSION"`
	}
	if err := json.Unmarshal(patched, &resource); err != nil {
		t.Fatal(err)
	}
	version := resource.Versions["#1"]["0000"]
	if version.Fixed["product_version"] != "2.7.9.0" || version.Fixed["file_version"] != "2.7.9.0" {
		t.Fatalf("prerelease text makes numeric Windows resource invalid: %+v", version.Fixed)
	}
	if version.Info["0409"]["ProductVersion"] != "2.7.9-Dev.4" || version.Info["0409"]["FileVersion"] != "2.7.9.0" || version.Info["0409"]["ProductName"] != "Installer" {
		t.Fatalf("friendly version/other metadata changed: %+v", version.Info)
	}
	if _, err := patchPublisherWinresVersion(data, "2.7.9\"bad"); err == nil {
		t.Fatal("invalid release version was accepted")
	}
}

func TestPublisherWindowsBuildUsesNativeScript(t *testing.T) {
	cmd := publisherBuildCommand(filepath.Join(t.TempDir(), "source with spaces"), "windows", true, true)
	if filepath.Base(cmd.Path) != "powershell.exe" && filepath.Base(cmd.Path) != "powershell" {
		t.Fatalf("Windows publisher requires its native PowerShell script, got %q %q", cmd.Path, cmd.Args)
	}
	if !strings.Contains(strings.Join(cmd.Args, "|"), "build.ps1") || strings.Contains(strings.Join(cmd.Args, "|"), "--cross-windows") {
		t.Fatalf("Windows must build natively without Bash or Linux cross-compiler flags: %q", cmd.Args)
	}
}

func TestPublisherLinuxCommandRetainsOptionalCrossBuild(t *testing.T) {
	cmd := publisherBuildCommand("/tmp/source with spaces", "linux", true, true)
	if len(cmd.Args) != 4 || cmd.Args[0] != "bash" || !strings.HasSuffix(cmd.Args[1], "build.sh") || cmd.Args[2] != "--clean" || cmd.Args[3] != "--cross-windows" {
		t.Fatalf("Linux build options changed: %q", cmd.Args)
	}
}

func TestPublisherCopiesPacksWithoutCP(t *testing.T) {
	source, destination := t.TempDir(), filepath.Join(t.TempDir(), "offline output")
	for name, content := range map[string]string{"UE_5.8/Content/Ünicode.uasset": "payload", ".pack.sha256": "sha"} {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if err := copyPublisherTree(source, destination); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(destination, "UE_5.8", "Content", "Ünicode.uasset")); err != nil || string(b) != "payload" {
		t.Fatalf("pack content lost: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(destination, ".pack.sha256")); err != nil {
		t.Fatal(err)
	}
	if err := copyPublisherTree(source, source); err != nil {
		t.Fatalf("already-in-place packs should not invoke an external copy: %v", err)
	}
	if err := copyPublisherTree(source, filepath.Join(source, "nested")); err == nil {
		t.Fatal("recursive copy into its own source must fail")
	}
}

func TestPublisherZIPInsideSourceDoesNotIncludeItself(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "gorgeous-installer.exe"), []byte("native"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(source, "release.zip")
	if err := os.WriteFile(output, []byte("old archive"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writePublisherZIP(source, output, false); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 1 || archive.File[0].Name != "gorgeous-installer.exe" {
		t.Fatalf("binary archive included itself or omitted the native EXE: %+v", archive.File)
	}
}

func TestPublisherWindowsArchiveHandlesPathCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows case-insensitive paths")
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "payload.txt"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	output := strings.ToUpper(filepath.Join(source, "release.zip"))
	if err := os.WriteFile(output, []byte("old archive"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writePublisherZIP(source, output, false); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 1 || archive.File[0].Name != "payload.txt" {
		t.Fatalf("case-variant output/temp archive included itself: %+v", archive.File)
	}
	if err := copyPublisherTree(source, strings.ToUpper(source)); err != nil {
		t.Fatalf("same Windows directory with different case must be a no-op: %v", err)
	}
}

func TestPublisherDirectoryLinksPreservePayload(t *testing.T) {
	source, content := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(content, "item.uasset"), []byte("linked asset"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(content, filepath.Join(source, "LinkedContent")); err != nil {
		t.Skipf("directory link creation unavailable: %v", err)
	}
	output := filepath.Join(t.TempDir(), "linked.zip")
	if err := writePublisherZIP(source, output, false); err != nil {
		t.Fatalf("existing linked payload must remain publishable: %v", err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	found := false
	for _, entry := range archive.File {
		if entry.Name == "LinkedContent/item.uasset" {
			found = true
		}
	}
	if !found {
		t.Fatal("directory link payload missing from portable archive")
	}
	target := t.TempDir()
	if err := copyPublisherTree(source, target); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(target, "LinkedContent", "item.uasset")); err != nil || string(b) != "linked asset" {
		t.Fatalf("linked copy lost payload: %q %v", b, err)
	}
	if err := os.Symlink(source, filepath.Join(source, "cycle")); err != nil {
		t.Fatal(err)
	}
	if err := writePublisherZIP(source, filepath.Join(t.TempDir(), "cycle.zip"), false); err == nil {
		t.Fatal("directory link cycle must report an error")
	}
}

func TestPublisherBuildStreamsPackageOutput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native PowerShell fixture")
	}
	dir := t.TempDir()
	script := "Write-Output 'package-alpha'\nStart-Sleep -Milliseconds 200\n[System.IO.File]::WriteAllText((Join-Path $PSScriptRoot 'finished'), 'yes')\n[Console]::Error.WriteLine('package-beta')\n"
	if err := os.WriteFile(filepath.Join(dir, "build.ps1"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	live := false
	var lines []string
	err := runPublisherBuild(publisherBuildCommand(dir, "windows", false, true), func(_ string, args ...any) {
		line := args[0].(string)
		lines = append(lines, line)
		if line == "package-alpha" {
			_, statErr := os.Stat(filepath.Join(dir, "finished"))
			live = os.IsNotExist(statErr)
		}
	})
	if err != nil || !live || strings.Join(lines, "|") != "package-alpha|package-beta" {
		t.Fatalf("build output was buffered/lost: live=%v lines=%q err=%v", live, lines, err)
	}
}

func TestNativeWindowsBuildScriptPackManifest(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows PowerShell5 compatibility")
	}
	buildScript, err := filepath.Abs(filepath.Join("..", "..", "build.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pack := filepath.Join(dir, "pack with spaces")
	if err := os.MkdirAll(filepath.Join(pack, "Content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "Content", "Ünicode.uasset"), []byte("actual-pack"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "manifest.sha256")
	harness := `param($BuildScript,$Pack,$Manifest)
$ErrorActionPreference='Stop'
$ast=[System.Management.Automation.Language.Parser]::ParseFile($BuildScript,[ref]$null,[ref]$null)
$functions=$ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -in @('Test-IsSHAControlFile','Resolve-RelativeToRoot','New-PackSHAManifest','Get-PackRelativePath') },$true)
foreach($function in $functions){ Invoke-Expression $function.Extent.Text }
New-PackSHAManifest -PackRoot $Pack -ManifestPath $Manifest
`
	path := filepath.Join(dir, "build.ps1")
	if err := os.WriteFile(path, []byte(harness), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := publisherBuildCommand(dir, "windows", false, false)
	cmd.Args = append(cmd.Args, buildScript, pack, manifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real build.ps1 manifest function failed on Windows PowerShell: %v\n%s", err, out)
	}
	bytes, err := os.ReadFile(manifest)
	if err != nil || !strings.Contains(string(bytes), "Content/Ünicode.uasset") {
		t.Fatalf("native pack manifest lost relative path: %q %v", bytes, err)
	}
}

func TestPublisherNativeWindowsBuildFixture(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires native Windows PowerShell")
	}
	dir := filepath.Join(t.TempDir(), "publisher build fixture")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	script := "param([switch]$Clean)\nif (-not $Clean) { exit 9 }\n[System.IO.File]::WriteAllText((Join-Path $PSScriptRoot 'native-built.txt'), 'windows-native')\n"
	if err := os.WriteFile(filepath.Join(dir, "build.ps1"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := publisherBuildCommand(dir, "windows", true, true).CombinedOutput(); err != nil {
		t.Fatalf("native publisher build command failed: %v\n%s", err, out)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "native-built.txt")); err != nil || string(b) != "windows-native" {
		t.Fatalf("native script did not run in its source directory: %q, %v", b, err)
	}
}

func TestPublisherArchiveWithoutExternalZIP(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source with spaces")
	for name, content := range map[string]string{"Content/Ünicode.txt": "payload", "build/old.exe": "exclude", ".git/config": "exclude", "tool.syso": "exclude", "trace.log": "exclude"} {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	destination := filepath.Join(t.TempDir(), "source.zip")
	if err := writePublisherZIP(source, destination, true); err != nil {
		t.Fatalf("publisher archive must work without external zip: %v", err)
	}
	archive, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	found := false
	for _, entry := range archive.File {
		if entry.Name == "Content/Ünicode.txt" {
			found = true
			continue
		}
		if !entry.FileInfo().IsDir() {
			t.Errorf("excluded installer source artifact was published: %s", entry.Name)
		}
	}
	if !found {
		t.Fatal("portable Unicode payload path missing from archive")
	}
}
