package ui

import (
	"encoding/json"
	"fmt"
	"io"
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

		go g.runOfflinePublish(win, versions, outDirEntry.Text, manifest, sourcePath, appendStatus)
	}, win)
	d.Resize(fyne.NewSize(550, 400))
	d.Show()
}

func (g *GUIApp) runOfflinePublish(win fyne.Window, versions []versionEntry, outDir string, manifest *SystemManifest, sourcePath string, appendStatus func(string, ...any)) {
	var progress *dialog.CustomDialog
	var statusLbl *widget.Label

	fyne.Do(func() {
		statusLbl = widget.NewLabel("Starting offline publish build for " + manifest.Name + "...")
		statusLbl.Wrapping = fyne.TextWrapWord
		progBar := widget.NewProgressBarInfinite()
		content := container.NewVBox(statusLbl, progBar)
		progress = dialog.NewCustom("Offline Publisher Progress", "Hide to Background", content, win)
		progress.Show()
	})

	updateStatus := func(msg string, args ...any) {
		text := fmt.Sprintf(msg, args...)
		appendStatus("%s", text)
		fyne.Do(func() {
			if statusLbl != nil {
				statusLbl.SetText(text)
			}
		})
	}

	updateStatus("Starting offline publish build for %s", manifest.Name)

	tempDir, err := os.MkdirTemp("", "gorgeous-offline-*")
	if err != nil {
		updateStatus("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	updateStatus("Cloning Gorgeous Installer repository...")
	cmdClone := exec.Command("git", "clone", "https://github.com/Epic-Nova/Gorgeous-Installer.git", tempDir)
	if err := cmdClone.Run(); err != nil {
		updateStatus("Git clone failed: %v", err)
		return
	}

	packsDir := filepath.Join(tempDir, "packs")
	os.MkdirAll(packsDir, 0755)

	var availVersions []config.PackVersion

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
	actualPluginName := filepath.Base(pluginRoot)

	for _, v := range versions {
		updateStatus("Packaging payload for UE %s (Sys %s)...", v.ueVer, v.sysVer)
		packName := fmt.Sprintf("%s-%s", manifest.ID, v.ueVer)
		packPath := filepath.Join(packsDir, packName)
		os.MkdirAll(packPath, 0755)

		var pathsToCopy []string
		if len(manifest.PayloadPaths) > 0 {
			for _, p := range manifest.PayloadPaths {
				pathsToCopy = append(pathsToCopy, filepath.Join(pluginRoot, p))
			}
		} else {
			pathsToCopy = []string{pluginRoot + "/."}
		}

		for _, src := range pathsToCopy {
			cmdCp := exec.Command("cp", "-R", src, packPath+"/")
			if err := cmdCp.Run(); err != nil {
				updateStatus("Copy failed for UE %s: %v", v.ueVer, err)
				return
			}
		}

		availVersions = append(availVersions, config.PackVersion{
			Version: v.ueVer,
			Path:    fmt.Sprintf("packs/%s", packName),
			SHAFile: fmt.Sprintf("packs/%s.sha256", packName),
		})
	}

	updateStatus("Generating config.json...")
	cfg := config.Config{
		PackName:          manifest.ID,
		PackType:          "hybrid",
		PluginName:        actualPluginName,
		AvailableVersions: availVersions,
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tempDir, "config.json"), cfgData, 0644)

	updateStatus("Compiling gorgeous-installer binary for Windows...")
	outExe := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s.exe", manifest.ID))
	
	cmdBuild := exec.Command("go", "build", "-ldflags", "-s -w", "-o", outExe, "./cmd/main")
	cmdBuild.Dir = tempDir
	cmdBuild.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=1", "CC=x86_64-w64-mingw32-gcc", "CXX=x86_64-w64-mingw32-g++")
	if _, err := cmdBuild.CombinedOutput(); err != nil {
		updateStatus("Windows build skipped or failed: %v\n(Note: Cross-compiling Fyne to Windows on Linux requires 'gcc-mingw-w64' installed)", err)
	} else {
		updateStatus("Windows build successful!")
	}

	updateStatus("Compiling gorgeous-installer binary via build.sh (Linux)...")
	outBin := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s", manifest.ID))
	cmdBuildLin := exec.Command("bash", "./build.sh")
	cmdBuildLin.Dir = tempDir
	if out, err := cmdBuildLin.CombinedOutput(); err != nil {
		updateStatus("Linux build failed: %v\n%s", err, string(out))
	} else {
		srcBin := filepath.Join(tempDir, "build", "gorgeous-installer")
		copyFile(srcBin, outBin)
		os.Chmod(outBin, 0755)
	}

	updateStatus("Offline publisher build completed! Files written to %s", outDir)

	fyne.Do(func() {
		if progress != nil {
			progress.Hide()
		}
		dialog.ShowInformation("Offline Publish Complete", fmt.Sprintf("Standalone installer successfully built!\n\nExecutables generated in:\n%s", outDir), win)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
