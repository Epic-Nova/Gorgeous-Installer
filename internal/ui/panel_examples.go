//go:build cgo

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gorgeous-installer/internal/installer"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// buildExamplePublisherPanel builds versioned standalone installers without a plugin manifest.
func (g *GUIApp) buildExamplePublisherPanel(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	name := widget.NewEntry()
	name.SetPlaceHolder("e.g. GorgeousLoadingScreenExample")
	kind := widget.NewSelect([]string{"Content Example", "Project Example"}, nil)
	kind.SetSelected("Content Example")
	version := widget.NewEntry()
	version.SetPlaceHolder("e.g. 5.7")
	source := widget.NewEntry()
	source.SetPlaceHolder("Content folder (contents only), or full project folder")
	var versions []versionEntry
	rows := container.NewVBox()
	var refresh func()
	refresh = func() {
		rows.Objects = nil
		for idx, v := range versions {
			index := idx
			rows.Add(container.NewBorder(nil, nil, nil, widget.NewButton("Remove", func() {
				versions = append(versions[:index], versions[index+1:]...)
				refresh()
			}), widget.NewLabel(fmt.Sprintf("UE %s: %s", v.ueVer, v.sourcePath))))
		}
		rows.Refresh()
	}
	kind.OnChanged = func(string) { versions = nil; refresh() }
	browse := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if uri != nil {
				source.SetText(normalizeURIPath(uri))
			}
		}, win)
	})
	add := widget.NewButton("Add version", func() {
		ue := strings.TrimSpace(version.Text)
		if !regexp.MustCompile(`^\d+\.\d+$`).MatchString(ue) {
			dialog.ShowError(fmt.Errorf("enter a UE version such as 5.7"), win)
			return
		}
		dir, err := filepath.Abs(strings.TrimSpace(source.Text))
		if strings.TrimSpace(source.Text) == "" {
			err = fmt.Errorf("select a source folder")
		}
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(dir)
			if err == nil && !info.IsDir() {
				err = fmt.Errorf("source must be a directory")
			}
		}
		if err == nil {
			packType := "content-example"
			if kind.Selected == "Project Example" {
				packType = "project-example"
			}
			err = installer.ValidateExamplePayload(packType, dir)
		}
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		for _, v := range versions {
			if v.ueVer == ue {
				dialog.ShowError(fmt.Errorf("UE %s is already mapped; remove it first", ue), win)
				return
			}
		}
		versions = append(versions, versionEntry{ueVer: ue, sourcePath: dir})
		refresh()
	})
	build := widget.NewButton("Build standalone installers", func() {
		id := strings.TrimSpace(name.Text)
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(id) {
			dialog.ShowError(fmt.Errorf("use letters, numbers, underscores or hyphens for the example name"), win)
			return
		}
		if len(versions) == 0 {
			dialog.ShowError(fmt.Errorf("add at least one UE version"), win)
			return
		}
		snapshot := append([]versionEntry(nil), versions...)
		g.showOfflinePublisherDialog(win, kind.Selected, &SystemManifest{ID: id, Name: id}, snapshot, "", "", appendStatus)
	})
	help := widget.NewLabel("Content Example copies the selected folder's contents into an existing project's Content folder. Preserve original asset paths and include dependencies. Project Example creates a complete project in an empty folder.")
	help.Wrapping = fyne.TextWrapWord
	return newGTCard("Example Installer Builder", "Package examples as standalone installers for website downloads",
		container.NewVBox(widget.NewLabel("Example name"), name, kind,
			help,
			container.NewBorder(nil, nil, widget.NewLabel("UE version"), nil, version),
			container.NewBorder(nil, nil, nil, browse, source), add, rows, build))
}
