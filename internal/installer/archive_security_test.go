package installer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZipValidatesAllPathsBeforeWriting(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, name := range []string{"kept.txt", "../escaped.txt"} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("replacement")); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	sentinel := filepath.Join(destination, "kept.txt")
	if err := os.WriteFile(sentinel, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, destination); err == nil {
		t.Fatal("traversal archive accepted")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "original" {
		t.Fatalf("archive modified installation before validation: %q %v", data, err)
	}
}
