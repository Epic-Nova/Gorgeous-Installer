//go:build cgo

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

type dropCaptureWindow struct {
	fyne.Window
	onDrop func(fyne.Position, []fyne.URI)
}

func (w *dropCaptureWindow) SetOnDropped(callback func(fyne.Position, []fyne.URI)) {
	w.onDrop = callback
}

func TestWindowDropUsesSelectionCallbackAndHonorsBusyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Spiel Projekt Ω%.UPROJECT")
	if err := os.WriteFile(path, []byte(`{"EngineAssociation":"5.8"}`), 0600); err != nil {
		t.Fatal(err)
	}
	w := &dropCaptureWindow{}
	busy := false
	var selections []string
	var failures []error
	configureProjectDrop(w, "content", func() bool { return busy }, func(path string) {
		selections = append(selections, path)
	}, func(err error) { failures = append(failures, err) })
	if w.onDrop == nil {
		t.Fatal("no native/Fyne window-wide file-drop callback installed")
	}
	w.onDrop(fyne.NewPos(20, 40), []fyne.URI{storage.NewFileURI(path)})
	if len(selections) != 1 || selections[0] != path || len(failures) != 0 {
		t.Fatalf("drop did not select exact Unicode/space/percent path: %v, %v", selections, failures)
	}
	busy = true
	w.onDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(path)})
	if len(selections) != 1 || len(failures) != 1 {
		t.Fatalf("busy drop changed target: %v, %v", selections, failures)
	}
	busy = false
	w.onDrop(fyne.Position{}, []fyne.URI{storage.NewFileURI(path), storage.NewFileURI(path)})
	if len(selections) != 1 || len(failures) != 2 {
		t.Fatal("ambiguous multi-file drop changed target")
	}
}

func TestWindowDropResolvesProjectFolderAndRejectsAmbiguity(t *testing.T) {
	folder := t.TempDir()
	drop := []fyne.URI{storage.NewFileURI(folder)}
	if _, err := droppedProjectPath("code", drop); err == nil {
		t.Fatal("folder without a project was accepted")
	}
	one := filepath.Join(folder, "One.uproject")
	if err := os.WriteFile(one, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if path, err := droppedProjectPath("code", drop); err != nil || path != one {
		t.Fatalf("single project folder: %q, %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Two.uproject"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := droppedProjectPath("code", drop); err == nil {
		t.Fatal("folder with multiple projects was accepted")
	}
}

func TestWindowDropValidatesExampleDestinationAndUnsupportedItems(t *testing.T) {
	folder := t.TempDir()
	if path, err := droppedProjectPath("project-example", []fyne.URI{storage.NewFileURI(folder)}); err != nil || path != folder {
		t.Fatalf("empty example destination: %q, %v", path, err)
	}
	file := filepath.Join(folder, "archive.zip")
	if err := os.WriteFile(file, []byte("not an installer request"), 0600); err != nil {
		t.Fatal(err)
	}
	remote, err := storage.ParseURI("https://example.invalid/Project.uproject")
	if err != nil {
		t.Fatal(err)
	}
	for _, items := range [][]fyne.URI{nil, {nil}, {storage.NewFileURI(file)}, {remote}, {storage.NewFileURI(filepath.Join(folder, "missing.uproject"))}} {
		if _, err := droppedProjectPath("content", items); err == nil {
			t.Fatalf("invalid drop accepted: %v", items)
		}
	}
	if _, err := droppedProjectPath("project-example", []fyne.URI{storage.NewFileURI(folder)}); err == nil {
		t.Fatal("nonempty example destination accepted")
	}
}
