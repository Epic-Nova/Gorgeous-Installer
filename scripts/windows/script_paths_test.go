package windowsscripts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repository(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func copyScript(t *testing.T, source, target string) {
	t.Helper()
	b, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func runScript(t *testing.T, script string, args ...string) ([]byte, error) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("native Windows PowerShell fixture")
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.Command(powershell, append([]string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script}, args...)...)
	cmd.Env = append(os.Environ(), "PSModulePath="+filepath.Join(filepath.Dir(powershell), "Modules")+string(os.PathListSeparator)+os.Getenv("PSModulePath"))
	return cmd.CombinedOutput()
}

func TestBuildCompatibilityWrapperForwardsAndReturnsStatus(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows fixture")
	}
	for _, tool := range []string{"go", "gcc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("existing script requires installed " + tool)
		}
	}
	root := t.TempDir()
	wrapper := filepath.Join(root, "scripts", "build.ps1")
	copyScript(t, filepath.Join(repository(t), "scripts", "build.ps1"), wrapper)
	stub := `param([switch]$Clean,[switch]$SkipUPX,[switch]$StrictPackSHA)
Write-Output ('canonical-build:'+ $Clean + ':' + $SkipUPX + ':' + $StrictPackSHA)
exit 17
`
	if err := os.WriteFile(filepath.Join(root, "build.ps1"), []byte(stub), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runScript(t, wrapper, "-Clean", "-SkipUPX", "-StrictPackSHA")
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 17 || !strings.Contains(string(out), "canonical-build:True:True:True") {
		t.Fatalf("wrapper did not delegate native switches/status: %v\n%s", err, out)
	}
}

func TestBashBuildCompatibilityWrapperForwardsLegacyFlags(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Git Bash fixture on Windows")
	}
	bash := filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	if _, err := os.Stat(bash); err != nil {
		t.Skip("Git Bash unavailable")
	}
	root := t.TempDir()
	path := filepath.Join(root, "scripts", "build.sh")
	copyScript(t, filepath.Join(repository(t), "scripts", "build.sh"), path)
	if err := os.WriteFile(filepath.Join(root, "build.sh"), []byte("#!/usr/bin/env bash\nprintf '%s\\n' \"$@\"\nexit 23\n"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bash, "--noprofile", "--norc", filepath.ToSlash(path), "--Clean", "--SkipUPX", "--cross-windows").CombinedOutput()
	if status, ok := err.(*exec.ExitError); !ok || status.ExitCode() != 23 || !strings.Contains(string(out), "--clean\n--skip-upx\n--cross-windows") {
		t.Fatalf("Bash wrapper did not preserve flags/status: %v\n%s", err, out)
	}
}

func TestSHAHelperUsesConfigDirectoryAndRestoresExactBytes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows fixture")
	}
	root := t.TempDir()
	pack := filepath.Join(root, "packs", "Unicode", "content")
	if err := os.MkdirAll(pack, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "sample.uasset"), []byte("fixture-payload"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "packs", "Unicode", "manifest.sha256")
	original := []byte{'o', 'r', 'i', 'g', 'i', 'n', 'a', 'l', 0xff, '\r', '\n'}
	if err := os.WriteFile(manifest, original, 0644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "custom-config.json")
	config := map[string]any{"packName": "Fixture", "packType": "content", "availableVersions": []map[string]string{{"version": "Fixture", "path": "packs/Unicode/content", "shaFile": "packs/Unicode/manifest.sha256"}}}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runScript(t, filepath.Join(repository(t), "scripts", "windows", "test-sha-comparison.ps1"), "-ConfigPath", configPath, "-Strict")
	if err != nil || !strings.Contains(string(out), "Validated failure count: 1") {
		t.Fatalf("real SHA helper did not validate the intended pack: %v\n%s", err, out)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("script changed original manifest bytes: %q, %v", after, err)
	}
}

func TestMetadataHelperResolvesRepositoryBuild(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows fixture")
	}
	root := t.TempDir()
	script := filepath.Join(root, "scripts", "windows", "show-exe-metadata.ps1")
	copyScript(t, filepath.Join(repository(t), "scripts", "windows", "show-exe-metadata.ps1"), script)
	if err := os.MkdirAll(filepath.Join(root, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "fixture.go")
	if err := os.WriteFile(source, []byte("package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"Gorgeous Installer script-fixture\")}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", filepath.Join(root, "build", "gorgeous-installer.exe"), source).CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v %s", err, out)
	}
	out, err := runScript(t, script)
	if err != nil || !strings.Contains(string(out), "Gorgeous Installer script-fixture") {
		t.Fatalf("metadata helper did not inspect repository build: %v\n%s", err, out)
	}
}

func TestSigningHelperChecksRepositoryBuildBeforeMissingCertificate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows fixture")
	}
	root := t.TempDir()
	script := filepath.Join(root, "scripts", "windows", "sign-build-output.ps1")
	copyScript(t, filepath.Join(repository(t), "scripts", "windows", "sign-build-output.ps1"), script)
	if err := os.MkdirAll(filepath.Join(root, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build", "gorgeous-installer.exe"), []byte("never executed or signed"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runScript(t, script, "-CertPath", filepath.Join(root, "absent.pfx"))
	if err == nil || !strings.Contains(string(out), "Certificate file not found:") {
		t.Fatalf("wrong default build path hides the intended missing-certificate error: %v\n%s", err, out)
	}
}

func TestWindowsRuntimeVersionMatchesResourceAndMetadata(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows PowerShell generation")
	}
	for _, fixture := range []struct{ product, friendly, file, expected string }{
		{"2.7.9.0", "1.0.0.0", "", "2.7.9"},
		{"2.7.9.0", "2.7.9-Dev.4", "", "2.7.9-Dev.4"},
		{"", "", "3.4.5.0", "3.4.5"},
	} {
		t.Run(fixture.expected, func(t *testing.T) {
			root := t.TempDir()
			fileVersion := fixture.file
			if fileVersion == "" {
				fileVersion = fixture.product
			}
			resource := map[string]any{"RT_VERSION": map[string]any{"#1": map[string]any{"0000": map[string]any{"fixed": map[string]string{"product_version": fixture.product, "file_version": fileVersion}, "info": map[string]any{"0409": map[string]string{"ProductVersion": fixture.friendly, "FileVersion": fileVersion}}}}}}
			data, _ := json.Marshal(resource)
			if err := os.WriteFile(filepath.Join(root, "winres.json"), data, 0644); err != nil {
				t.Fatal(err)
			}
			harness := `param($BuildScript,$FixtureRoot)
$ErrorActionPreference='Stop'
$text=[IO.File]::ReadAllText($BuildScript)
$ast=[System.Management.Automation.Language.Parser]::ParseFile($BuildScript,[ref]$null,[ref]$null)
foreach($function in $ast.FindAll({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Get-InstallerRuntimeVersion'},$true)){Invoke-Expression $function.Extent.Text}
$scriptDir=$FixtureRoot; $gitCommit='fixture'; $sha256='fixture-sha'; $Sign=$false
$metaPath=Join-Path $FixtureRoot 'metadata.json'
Push-Location $FixtureRoot
try {
 $start=$text.IndexOf('$buildTimeUtc ='); $end=$text.IndexOf('$ldflags =',$start)
 Invoke-Expression $text.Substring($start,$end-$start)
 $start=$text.IndexOf('$goVersion ='); $end=$text.IndexOf('$fileSize =',$start)
 Invoke-Expression $text.Substring($start,$end-$start)
} finally {Pop-Location}
`
			path := filepath.Join(root, "generation-fixture.ps1")
			if err := os.WriteFile(path, []byte(harness), 0644); err != nil {
				t.Fatal(err)
			}
			if out, err := runScript(t, path, filepath.Join(repository(t), "build.ps1"), root); err != nil {
				t.Fatalf("actual build generation: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.26.2\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nimport(\"fmt\";\"fixture/internal/buildinfo\")\nfunc main(){fmt.Println(buildinfo.Summary())}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(root, "version.exe")
			cmd := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", exe, ".")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compile actual generated buildinfo: %v\n%s", err, out)
			}
			if out, err := exec.Command(exe, "-version-info").CombinedOutput(); err != nil || !strings.Contains(string(out), "Gorgeous Installer "+fixture.expected+" (") {
				t.Fatalf("runtime reports a stale release version: %v %s", err, out)
			}
			metadataBytes, err := os.ReadFile(filepath.Join(root, "metadata.json"))
			if err != nil {
				t.Fatal(err)
			}
			var metadata map[string]any
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata["version"] != fixture.expected {
				t.Fatalf("metadata version differs: %+v", metadata)
			}
		})
	}
}

func TestWindowsBuildConfigRemainsGoJSONCompatible(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows PowerShell encoding")
	}
	root := t.TempDir()
	harness := `param($BuildScript,$FixtureRoot)
$ErrorActionPreference='Stop'
$text=[IO.File]::ReadAllText($BuildScript)
$start=$text.IndexOf('if ($configUpdated) {');$end=$text.IndexOf('if ($skippedSHAVersions.Count',$start)
$configUpdated=$true;$scriptDir=$FixtureRoot
$configData='{"packName":"Fixture","availableVersions":[{"version":"5.8","path":"packs/5.8/content","shaFile":"packs/5.8/manifest.sha256"}]}'|ConvertFrom-Json
Push-Location $FixtureRoot
try {Invoke-Expression $text.Substring($start,$end-$start)} finally {Pop-Location}
`
	path := filepath.Join(root, "config-generation.ps1")
	if err := os.WriteFile(path, []byte(harness), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := runScript(t, path, filepath.Join(repository(t), "build.ps1"), root); err != nil {
		t.Fatalf("actual config generation: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("build.ps1 produced JSON that the actual Go config reader cannot decode: %v, prefix=%x", err, data[:min(4, len(data))])
	}
}
