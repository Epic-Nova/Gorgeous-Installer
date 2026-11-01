//go:build cgo

package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
)

func TestExampleGUIInstallUsesSelectedVersionAndProjectContent(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Game.uproject")
	if err := os.WriteFile(project, []byte(`{"EngineAssociation":"5.7"}`), 0644); err != nil {
		t.Fatal(err)
	}
	var versions []config.PackVersion
	for _, v := range []string{"5.6", "5.7"} {
		source := filepath.Join(root, "payload-"+v)
		if err := os.MkdirAll(filepath.Join(source, "Example"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "Example/Map.umap"), []byte(v), 0644); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, config.PackVersion{Version: v, Path: source})
	}
	g := &GUIApp{config: &config.Config{PackType: "content-example", InstallPath: "Content", AvailableVersions: versions}}
	plan, err := g.inspectInstallPlan(project, "5.6")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PackVersion != "5.6" || plan.DestinationRoot != filepath.Join(root, "Content") {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if err := g.performInstall(context.Background(), project, "5.6", installer.InstallActionInstall, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "Content/Example/Map.umap")); err != nil || string(data) != "5.6" {
		t.Fatalf("wrong payload: %q, %v", data, err)
	}
}
