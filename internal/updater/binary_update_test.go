package updater

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryUpdatePreservesExistingTemporaryDirectory(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("TMP", scratch)
	t.Setenv("TEMP", scratch)
	t.Setenv("TMPDIR", scratch)
	existing := filepath.Join(scratch, "gorgeous-installer-extracted", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(existing), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("another update owns this"), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(scratch, "missing-executable.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	entry, err := w.Create("readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("no binary")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := applyBinaryUpdate(archive, filepath.Join(scratch, "installer.exe")); err == nil {
		t.Fatal("missing executable accepted")
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "another update owns this" {
		t.Fatalf("unrelated temporary files were removed: %q %v", data, err)
	}
}
