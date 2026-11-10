package unreal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestProjectFileGeneratorCommand(t *testing.T) {
	for _, tc := range []struct {
		name, platform, generator string
		ubt                       bool
	}{
		{"installed Windows engine", "windows", "Engine/Binaries/DotNET/UnrealBuildTool/UnrealBuildTool.exe", true},
		{"source Windows engine fallback", "windows", "Engine/Build/BatchFiles/GenerateProjectFiles.bat", false},
		{"Linux engine", "linux", "Engine/Build/BatchFiles/Linux/GenerateProjectFiles.sh", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := filepath.Join(t.TempDir(), "Engine With Spaces")
			generator := filepath.Join(engine, filepath.FromSlash(tc.generator))
			if err := os.MkdirAll(filepath.Dir(generator), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(generator, nil, 0755); err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(t.TempDir(), "Project With Spaces", "Sample.uproject")
			cmd, err := projectFileGeneratorCommand(context.Background(), engine, project, tc.platform)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{generator, "-project=" + project, "-game"}
			if tc.ubt {
				want = []string{generator, "-projectfiles", "-project=" + project, "-game", "-engine"}
			}
			if !reflect.DeepEqual(cmd.Args, want) {
				t.Fatalf("args = %#v, want %#v", cmd.Args, want)
			}
		})
	}
	if _, err := projectFileGeneratorCommand(context.Background(), t.TempDir(), "Missing.uproject", "windows"); err == nil {
		t.Fatal("missing generator was accepted")
	}
}

// Opt in on a workstation with an installed Unreal engine; ordinary CI needs no engine.
func TestGenerateProjectFilesInstalledEngine(t *testing.T) {
	engine := os.Getenv("GORGEOUS_TEST_ENGINE_PATH")
	if engine == "" || runtime.GOOS != "windows" {
		t.Skip("set GORGEOUS_TEST_ENGINE_PATH to run the native Windows Unreal check")
	}
	base := t.TempDir()
	if testRoot := os.Getenv("GORGEOUS_TEST_PROJECT_ROOT"); testRoot != "" {
		if err := os.MkdirAll(testRoot, 0755); err != nil {
			t.Fatal(err)
		}
		var err error
		base, err = os.MkdirTemp(testRoot, "smoke-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(base) })
	}
	root := filepath.Join(base, "Project With Spaces")
	if err := os.MkdirAll(filepath.Join(root, "Source", "InstallerSmoke"), 0755); err != nil {
		t.Fatal(err)
	}
	descriptor, err := json.Marshal(map[string]any{"FileVersion": 3, "EngineAssociation": engine, "Modules": []map[string]string{{"Name": "InstallerSmoke", "Type": "Runtime", "LoadingPhase": "Default"}}})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"InstallerSmoke.uproject":                       string(descriptor),
		"Source/InstallerSmoke/InstallerSmoke.Build.cs": "using UnrealBuildTool; public class InstallerSmoke : ModuleRules { public InstallerSmoke(ReadOnlyTargetRules Target) : base(Target) { PublicDependencyModuleNames.AddRange(new string[] {\"Core\", \"CoreUObject\", \"Engine\"}); } }",
		"Source/InstallerSmoke.Target.cs":               "using UnrealBuildTool; public class InstallerSmokeTarget : TargetRules { public InstallerSmokeTarget(TargetInfo Target) : base(Target) { Type = TargetType.Game; DefaultBuildSettings = BuildSettingsVersion.Latest; ExtraModuleNames.Add(\"InstallerSmoke\"); } }",
		"Source/InstallerSmokeEditor.Target.cs":         "using UnrealBuildTool; public class InstallerSmokeEditorTarget : TargetRules { public InstallerSmokeEditorTarget(TargetInfo Target) : base(Target) { Type = TargetType.Editor; DefaultBuildSettings = BuildSettingsVersion.Latest; ExtraModuleNames.Add(\"InstallerSmoke\"); } }",
		"Source/InstallerSmoke/InstallerSmoke.cpp":      "#include \"Modules/ModuleManager.h\"\nIMPLEMENT_PRIMARY_GAME_MODULE(FDefaultGameModuleImpl, InstallerSmoke, \"InstallerSmoke\");\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := GenerateProjectFiles(ctx, filepath.Join(root, "InstallerSmoke.uproject"), t.Logf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "InstallerSmoke.sln")); err != nil {
		t.Fatalf("project solution was not generated: %v", err)
	}
	if os.Getenv("GORGEOUS_TEST_COMPILE_PROJECT") == "1" {
		buildCtx, stopBuild := context.WithTimeout(context.Background(), 10*time.Minute)
		defer stopBuild()
		if err := BuildProject(buildCtx, filepath.Join(root, "InstallerSmoke.uproject"), t.Logf); err != nil {
			t.Fatal(err)
		}
		if !CheckProjectBinaries(filepath.Join(root, "InstallerSmoke.uproject")) {
			t.Fatal("built editor module does not match the engine")
		}
	}
}
