package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gorgeous-installer/internal/buildinfo"
	"gorgeous-installer/internal/settings"
	"gorgeous-installer/internal/updater"
	"os"
	"runtime"
	"time"
)

func (g *GUIApp) buildSettingsPanel(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	appSettings, err := settings.LoadSettings()
	if err != nil {
		appendStatus("Failed to load settings: %v", err)
		appSettings = settings.DefaultSettings()
	}

	// Paths list
	pathsVBox := container.NewVBox()
	refreshPaths := func() {
		pathsVBox.Objects = nil
		for i, p := range appSettings.SearchPaths {
			idx := i
			pathLbl := newGTLabel(p)
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				appSettings.SearchPaths = append(appSettings.SearchPaths[:idx], appSettings.SearchPaths[idx+1:]...)
				pathsVBox.Refresh()
			})
			delBtn.Importance = widget.DangerImportance
			row := container.NewHBox(pathLbl, layout.NewSpacer(), delBtn)
			pathsVBox.Add(row)
		}
		if len(appSettings.SearchPaths) == 0 {
			pathsVBox.Add(newGTLabel("No search paths configured."))
		}
		pathsVBox.Refresh()
	}
	refreshPaths()

	addPathBtn := widget.NewButtonWithIcon("Add Path", theme.FolderOpenIcon(), func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			p := normalizeURIPath(lu)
			if p != "" {
				appSettings.SearchPaths = append(appSettings.SearchPaths, p)
				refreshPaths()
			}
		}, win)
		d.Show()
	})

	pathsSection := newGTCard("Search Paths", "Directories to scan recursively for Unreal projects",
		container.NewVBox(
			container.NewPadded(pathsVBox),
			container.NewHBox(addPathBtn, layout.NewSpacer()),
		),
	)

	// Installer Section
	binPathLbl := newGTValueLabel(appSettings.LocalBinPath)
	changeBinBtn := widget.NewButtonWithIcon("Change Bin Path", theme.FolderOpenIcon(), func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			p := normalizeURIPath(lu)
			if p != "" {
				appSettings.LocalBinPath = p
				setCanvasText(binPathLbl, p)
			}
		}, win)
		d.Show()
	})

	installerVBox := container.NewVBox(
		container.NewHBox(newGTLabel("Local Bin Directory:"), binPathLbl),
		container.NewHBox(changeBinBtn, layout.NewSpacer()),
	)
		
	if runtime.GOOS == "windows" {
		assocCheck := widget.NewCheck("Associate .uproject files with Gorgeous Installer", func(b bool) {
			appSettings.UprojectAssociated = b
		})
		assocCheck.SetChecked(appSettings.UprojectAssociated)
		installerVBox.Add(assocCheck)
	}

	installerSection := newGTCard("Installer Configuration", "Native installation settings",
		installerVBox,
	)

	// Update Section
	var updateSection fyne.CanvasObject
	if appSettings.InstalledNatively {
		newVer, ok := updater.CheckForUpdates(buildinfo.Version)
		if ok {
			updateBtn := newAccentButton("Update to v"+newVer, accentUpdate, func() {
				err := updater.PerformUpdate(appSettings.LocalBinPath + "/gorgeous-installer")
				if err == nil {
					appendStatus("Update started. Installer will now restart.")
					time.Sleep(500 * time.Millisecond)
					os.Exit(0)
				} else {
					appendStatus("Update failed: %v", err)
				}
			})
			updateSection = newGTCard("Update Available", "A newer version of Gorgeous Installer is available", container.NewVBox(updateBtn))
		} else {
			updateSection = container.NewVBox()
		}
	} else {
		updateSection = container.NewVBox()
	}

	// Save button
	saveBtn := newAccentButton("Save Settings", accentUpdate, func() {
		// Attempt to update the registry if on Windows
		if err := settings.UpdateRegistryAssociation(appSettings); err != nil {
			g.showAnimatedDialog("Registry Error", "Failed to update Windows registry: "+err.Error(), true)
			appendStatus("Registry Error: %v", err)
			// Continue saving settings anyway so the user's changes to path/etc aren't lost
		}

		if err := settings.SaveSettings(appSettings); err != nil {
			g.showAnimatedDialog("Error", err.Error(), true)
			appendStatus("Failed to save settings: %v", err)
			return
		}
		appendStatus("Settings saved successfully to %s", ".config/GorgeousThings/Installer.json")
		g.showAnimatedDialog("Success", "Settings saved successfully", false)
	})

	content := container.NewVBox(
		updateSection,
		pathsSection,
		installerSection,
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(200, 50), saveBtn)),
	)

	return container.NewPadded(container.NewScroll(content))
}
