package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceDirectoryDoesNotUpdateUnrelatedLaunchDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "installer source")
	game := filepath.Join(root, "Game")
	for _, p := range []string{filepath.Join(source, "build"), game} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module gorgeous-installer\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game, "go.mod"), []byte("module unrelated-game\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := SourceDirectory(filepath.Join(source, "build", "installer.exe"), game)
	if err != nil || got != source {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = SourceDirectory(filepath.Join(root, "go-run", "temporary.exe"), source)
	if err != nil || got != source {
		t.Fatalf("go-run fallback: %q %v", got, err)
	}
	if _, err := SourceDirectory(filepath.Join(root, "unrelated.exe"), game); err == nil {
		t.Fatal("unrelated project accepted as installer source")
	}
}
