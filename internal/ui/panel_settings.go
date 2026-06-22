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

	// Developer Section
	// Determine context
	isDev := appSettings.DevMode
	if appSettings.InstalledNatively {
		isDev = appSettings.BinDevMode
	}

	forceHttpCheck := widget.NewCheck("Force HTTP (Local Testing)", func(b bool) {
		appSettings.ForceHTTP = b
	})
	forceHttpCheck.SetChecked(appSettings.ForceHTTP)
	if !isDev {
		forceHttpCheck.Hide()
	}

	updateDevUI := func() {
		anyDev := appSettings.DevMode || appSettings.BinDevMode
		if anyDev {
			forceHttpCheck.Show()
			if g.navItemsBox != nil && g.navPublisherBtn != nil {
				found := false
				for _, obj := range g.navItemsBox.Objects {
					if obj == g.navPublisherBtn {
						found = true
						break
					}
				}
				if !found {
					g.navItemsBox.Add(g.navPublisherBtn)
				}
			}
		} else {
			forceHttpCheck.Hide()
			appSettings.ForceHTTP = false
			forceHttpCheck.SetChecked(false)
			if g.navItemsBox != nil && g.navPublisherBtn != nil {
				g.navItemsBox.Remove(g.navPublisherBtn)
			}
		}
	}

	devModeCheck := widget.NewCheck("Enable Developer Mode (Plugin Source)", func(b bool) {
		appSettings.DevMode = b
		updateDevUI()
	})
	devModeCheck.SetChecked(appSettings.DevMode)

	binDevModeCheck := widget.NewCheck("Enable Developer Mode (Standalone Installer)", func(b bool) {
		appSettings.BinDevMode = b
		updateDevUI()
	})
	binDevModeCheck.SetChecked(appSettings.BinDevMode)

	devHelpBtn := widget.NewButtonWithIcon("", theme.QuestionIcon(), func() {
		g.showAnimatedDialog("Developer Mode", "Enabling Developer Mode allows testing HTTP connections and reveals the Publisher Menu.\n\nWhen enabled on a local codebase, Installer Source Updates are disabled to prevent overwriting your local edits.", false)
	})

	devVBox := container.NewVBox(
		container.NewHBox(devModeCheck, devHelpBtn),
		binDevModeCheck,
		forceHttpCheck,
	)

	devSection := newGTCard("Developer Settings", "Advanced settings for debugging and development",
		devVBox,
	)

	// Update Section
	var updateSection fyne.CanvasObject
	skipUpdateCheck := isDev && !appSettings.InstalledNatively
	if !skipUpdateCheck {
		newVer, ok := updater.CheckForUpdates(buildinfo.Version, appSettings.InstalledNatively)
		if ok {
			updateBtn := newAccentButton("Update to v"+newVer, accentUpdate, func() {
				binPath := appSettings.LocalBinPath + "/gorgeous-installer"
				if runtime.GOOS == "windows" {
					binPath += ".exe"
				}
				if !appSettings.InstalledNatively {
					binPath = "." // for source update, extract into current dir
				}
				err := updater.PerformUpdate(binPath, appSettings.InstalledNatively)
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
		devSection,
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(200, 50), saveBtn)),
	)

	return container.NewPadded(container.NewScroll(content))
}
