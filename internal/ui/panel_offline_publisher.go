package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gorgeous-installer/internal/config"
)

type versionEntry struct {
	ueVer   string
	sysVer  string
}

func (g *GUIApp) showOfflinePublisherDialog(win fyne.Window, manifest *SystemManifest, sourcePath string, appendStatus func(string, ...any)) {
	if manifest == nil {
		return
	}

	entriesBox := container.NewVBox()

	var versions []versionEntry

	var updateList func()
	updateList = func() {
		entriesBox.Objects = nil
		for i, v := range versions {
			idx := i
			lbl := widget.NewLabel(fmt.Sprintf("Engine %s -> Sys v%s", v.ueVer, v.sysVer))
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				versions = append(versions[:idx], versions[idx+1:]...)
				updateList()
			})
			entriesBox.Add(container.NewHBox(lbl, layout.NewSpacer(), delBtn))
		}
		entriesBox.Refresh()
	}

	ueVerEntry := widget.NewEntry()
	ueVerEntry.SetPlaceHolder("e.g. 5.4")
	
	sysVerEntry := widget.NewEntry()
	sysVerEntry.SetPlaceHolder("System version, e.g. 1.0.0")
	sysVerEntry.SetText(manifest.Version)

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		if ueVerEntry.Text == "" || sysVerEntry.Text == "" {
			return
		}
		versions = append(versions, versionEntry{ueVer: ueVerEntry.Text, sysVer: sysVerEntry.Text})
		ueVerEntry.SetText("")
		updateList()
	})

	inputRow := container.NewHBox(
		widget.NewLabel("UE Ver:"), container.NewGridWrap(fyne.NewSize(60, 35), ueVerEntry),
		widget.NewLabel("Sys Ver:"), container.NewGridWrap(fyne.NewSize(80, 35), sysVerEntry),
		addBtn,
	)

	outDirEntry := widget.NewEntry()
	outDirEntry.SetPlaceHolder("Output Directory")
	
	browseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if lu != nil {
				outDirEntry.SetText(lu.Path())
			}
		}, win)
	})

	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Offline Builder for %s", manifest.Name)),
		widget.NewLabel("Map Engine versions to System versions for the installer:"),
		container.NewPadded(container.NewVScroll(entriesBox)),
		inputRow,
		widget.NewSeparator(),
		container.NewHBox(widget.NewLabel("Output:"), container.NewGridWrap(fyne.NewSize(300, 35), outDirEntry), browseBtn),
	)

	d := dialog.NewCustomConfirm("Offline Publishing", "Build Installer", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		if len(versions) == 0 {
			dialog.ShowError(fmt.Errorf("please add at least one version mapping"), win)
			return
		}
		if outDirEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("please select an output directory"), win)
			return
		}

		go g.runOfflinePublish(versions, outDirEntry.Text, manifest, sourcePath, appendStatus)
	}, win)
	d.Resize(fyne.NewSize(550, 400))
	d.Show()
}

func (g *GUIApp) runOfflinePublish(versions []versionEntry, outDir string, manifest *SystemManifest, sourcePath string, appendStatus func(string, ...any)) {
	appendStatus("Starting offline publish build for %s", manifest.Name)

	tempDir, err := os.MkdirTemp("", "gorgeous-offline-*")
	if err != nil {
		appendStatus("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	appendStatus("Cloning Gorgeous Installer repository...")
	cmdClone := exec.Command("git", "clone", "https://github.com/Epic-Nova/Gorgeous-Installer.git", tempDir)
	if err := cmdClone.Run(); err != nil {
		appendStatus("Git clone failed: %v", err)
		return
	}

	packsDir := filepath.Join(tempDir, "packs")
	os.MkdirAll(packsDir, 0755)

	var availVersions []config.PackVersion

	for _, v := range versions {
		appendStatus("Packaging payload for UE %s (Sys %s)...", v.ueVer, v.sysVer)
		zipName := fmt.Sprintf("%s-%s.zip", manifest.ID, v.ueVer)
		zipPath := filepath.Join(packsDir, zipName)

		pluginRoot := sourcePath
		for pluginRoot != "" && pluginRoot != string(filepath.Separator) && pluginRoot != "." {
			matches, _ := filepath.Glob(filepath.Join(pluginRoot, "*.uplugin"))
			if len(matches) > 0 {
				break
			}
			parent := filepath.Dir(pluginRoot)
			if parent == pluginRoot {
				pluginRoot = sourcePath
				break
			}
			pluginRoot = parent
		}

		args := []string{"-r", zipPath}
		if len(manifest.PayloadPaths) > 0 {
			args = append(args, manifest.PayloadPaths...)
		} else {
			rel, _ := filepath.Rel(pluginRoot, sourcePath)
			if rel != "." && rel != "" {
				args = append(args, rel)
			} else {
				args = append(args, ".")
			}
		}

		cmdZip := exec.Command("zip", args...)
		cmdZip.Dir = pluginRoot
		if err := cmdZip.Run(); err != nil {
			appendStatus("Zip failed for UE %s: %v", v.ueVer, err)
			return
		}

		availVersions = append(availVersions, config.PackVersion{
			Version: v.ueVer,
			Path:    fmt.Sprintf("packs/%s", zipName),
		})
	}

	appendStatus("Generating config.json...")
	cfg := config.Config{
		PackName:          manifest.ID,
		PackType:          "hybrid",
		PluginName:        manifest.Name,
		AvailableVersions: availVersions,
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tempDir, "config.json"), cfgData, 0644)

	appendStatus("Compiling gorgeous-installer binary...")
	outExe := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s.exe", manifest.ID))
	
	cmdBuild := exec.Command("go", "build", "-ldflags", "-s -w", "-o", outExe, "./cmd/main")
	cmdBuild.Dir = tempDir
	cmdBuild.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		appendStatus("Build failed: %v\n%s", err, string(out))
		return
	}

	outBin := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s", manifest.ID))
	cmdBuildLin := exec.Command("go", "build", "-ldflags", "-s -w", "-o", outBin, "./cmd/main")
	cmdBuildLin.Dir = tempDir
	cmdBuildLin.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmdBuildLin.CombinedOutput(); err != nil {
		appendStatus("Linux build failed: %v\n%s", err, string(out))
	}

	appendStatus("Offline publisher build completed! Files written to %s", outDir)
}
