//go:build cgo

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const windowDragHeight = float32(28)

// Reserve a non-interactive strip so native caption hit-testing never takes
// clicks from the install controls, even when a panel replaces the content.
type dragAreaWindow struct{ fyne.Window }

func withWindowDragArea(window fyne.Window) fyne.Window {
	if runtime.GOOS != "windows" {
		return window
	}
	return &dragAreaWindow{Window: window}
}

func (w *dragAreaWindow) SetContent(content fyne.CanvasObject) {
	background := canvas.NewRectangle(gtBg0)
	background.SetMinSize(fyne.NewSize(0, windowDragHeight))
	grip := canvas.NewText("• • •", gtTextDim)
	grip.TextSize = 10
	header := container.NewStack(background, container.NewCenter(grip))
	w.Window.SetContent(container.NewBorder(header, nil, nil, nil, content))
}

func configureProjectDrop(window fyne.Window, packType string, busy func() bool, selectProject func(string), reportError func(error)) {
	window.SetOnDropped(func(_ fyne.Position, items []fyne.URI) {
		if busy != nil && busy() {
			reportError(fmt.Errorf("wait for the current installation to finish before dropping a project"))
			return
		}
		selected, err := droppedProjectPath(packType, items)
		if err != nil {
			reportError(err)
			return
		}
		// A drop selects the same target as Browse; it never starts installation.
		selectProject(selected)
	})
}

func droppedProjectPath(packType string, items []fyne.URI) (string, error) {
	if len(items) != 1 || items[0] == nil || items[0].Scheme() != "file" {
		return "", fmt.Errorf("drop one local .uproject file or project folder")
	}
	if authority := items[0].Authority(); authority != "" && !strings.EqualFold(authority, "localhost") {
		return "", fmt.Errorf("drop a local file or folder")
	}
	path := normalizeURIPath(items[0])
	if strings.TrimSpace(items[0].Path()) == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("the dropped item must be a local absolute path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("cannot open dropped item: %w", err)
	}
	if packType == "project-example" {
		if !info.IsDir() {
			return "", fmt.Errorf("drop an empty destination folder for the example project")
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return "", fmt.Errorf("the example project destination must be an empty readable folder")
		}
		return path, nil
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(path), ".uproject") {
			return "", fmt.Errorf("drop a .uproject file or its project folder")
		}
		return path, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("cannot read dropped folder: %w", err)
	}
	var projects []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".uproject") {
			projects = append(projects, filepath.Join(path, entry.Name()))
		}
	}
	if len(projects) != 1 {
		return "", fmt.Errorf("the dropped folder must contain exactly one .uproject file; drop the desired file directly")
	}
	return projects[0], nil
}
