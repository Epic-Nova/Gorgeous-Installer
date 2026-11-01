package installer

import (
	"os"
	"path/filepath"
	"testing"

	"gorgeous-installer/internal/config"
)

func writeExampleFile(t *testing.T, root, rel, value string) {
	t.Helper()
	file := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestContentExampleInstallsAndUpdatesOnlyPackFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "payload")
	project := filepath.Join(root, "game")
	writeExampleFile(t, project, "Game.uproject", `{"EngineAssociation":"5.7"}`)
	writeExampleFile(t, project, "Content/User/Keep.uasset", "user")
	writeExampleFile(t, source, "LoadingExample/Map.umap", "v1")
	inst := NewInstaller("", "content-example", &config.PackVersion{Version: "5.7", Path: source}, "Content", filepath.Join(project, "Game.uproject"), "")
	if err := inst.Install(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, "Content/LoadingExample/Map.umap")
	if data, err := os.ReadFile(target); err != nil || string(data) != "v1" {
		t.Fatalf("installed asset: %q, %v", data, err)
	}
	writeExampleFile(t, source, "LoadingExample/Map.umap", "v2")
	inst.SetInstallAction(InstallActionUpdate)
	if err := inst.Install(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "v2" {
		t.Fatalf("update failed: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(project, "Content/User/Keep.uasset")); string(data) != "user" {
		t.Fatal("unrelated asset was changed")
	}
	if _, err := os.Stat(filepath.Join(project, "Plugins")); !os.IsNotExist(err) {
		t.Fatal("content example must not create a plugin")
	}
}

func TestContentExampleRejectsInvalidTargetsAndWrappedPayloads(t *testing.T) {
	for _, rel := range []string{"Content/Example/Map.umap", "Game.uproject", "Config/DefaultEngine.ini"} {
		t.Run(rel, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "payload")
			writeExampleFile(t, root, "Game.uproject", `{}`)
			writeExampleFile(t, source, rel, "asset")
			inst := NewInstaller("", "content-example", &config.PackVersion{Path: source}, "Content", filepath.Join(root, "Game.uproject"), "")
			if _, err := inst.BuildInstallPlan(); err == nil {
				t.Fatal("accepted project-root payload as Content contents")
			}
		})
	}
	inst := NewInstaller("", "content-example", &config.PackVersion{Path: t.TempDir()}, "Content", "missing.uproject", "")
	if _, err := inst.BuildInstallPlan(); err == nil {
		t.Fatal("accepted missing project")
	}
}

func TestFullProjectExampleRequiresEmptyDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "payload")
	target := filepath.Join(root, "new-game")
	writeExampleFile(t, source, "Example.uproject", `{"EngineAssociation":"5.7"}`)
	writeExampleFile(t, source, "Content/Example/Map.umap", "map")
	inst := NewInstaller("", "project-example", &config.PackVersion{Path: source}, "", target, "")
	if err := inst.Install(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "Example.uproject")); err != nil {
		t.Fatal(err)
	}
	if err := inst.Install(); err == nil {
		t.Fatal("overwrote populated project directory")
	}
}
