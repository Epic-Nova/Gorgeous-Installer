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
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func (g *GUIApp) buildSettingsPanel(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	appSettings, err := settings.LoadSettings()
	if err != nil {
		appendStatus("Failed to load settings: %v", err)
		appSettings = settings.DefaultSettings()
	}

	networkCheck := widget.NewCheck("Allow Gorgeous API access (news, catalog, updates)", func(allowed bool) {
		appSettings.AllowNetwork = allowed
		appSettings.NetworkConsentDecided = true
		if allowed {
			os.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
		} else {
			os.Unsetenv("GORGEOUS_ALLOW_NETWORK")
		}
		_ = settings.SaveSettings(appSettings)
	})
	networkCheck.SetChecked(appSettings.AllowNetwork)
	privacyURL, _ := url.Parse("https://epicnova.net/legal/privacy-policy/long")
	networkSection := newGTCard("Network and privacy", "Consent is stored for this user; --offline overrides it", container.NewVBox(networkCheck, widget.NewHyperlink("Privacy policy", privacyURL)))

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
		container.NewHBox(newGTLabel("Installed Version:"), newGTValueLabel(buildinfo.Version)),
	)
	if runtime.GOOS == "windows" {
		installBtn := widget.NewButtonWithIcon("Install or repair for this user", theme.DownloadIcon(), nil)
		uninstallBtn := widget.NewButtonWithIcon("Uninstall from this user account", theme.DeleteIcon(), nil)
		if !settings.NativeInstallationPresent(appSettings) {
			uninstallBtn.Disable()
		}
		installBtn.OnTapped = func() {
			snapshot := *appSettings
			snapshot.SearchPaths = append([]string(nil), appSettings.SearchPaths...)
			progress := dialog.NewProgressInfinite("Installing Gorgeous Installer", "Adding this user's Start menu shortcut and Installed Apps entry...", win)
			progress.Show()
			go func() {
				err := settings.InstallNative(&snapshot, buildinfo.Version)
				fyne.Do(func() {
					progress.Hide()
					if err != nil {
						g.showAnimatedDialog("Installation failed", err.Error(), true)
						return
					}
					appSettings.InstalledNatively = snapshot.InstalledNatively
					uninstallBtn.Enable()
					appendStatus("Installed for this user in %s; no autostart was enabled.", appSettings.LocalBinPath)
					g.showAnimatedDialog("Installed", "Gorgeous Installer is available in the Start menu and Windows Installed Apps.", false)
				})
			}()
		}
		uninstallBtn.OnTapped = func() {
			dialog.ShowConfirm("Uninstall Gorgeous Installer", "Remove this user's installed executable, Start menu shortcut and Installed Apps entry? Your preferences and unrelated files are kept.", func(confirmed bool) {
				if !confirmed {
					return
				}
				mustExit, err := settings.UninstallNative(appSettings)
				if err != nil {
					g.showAnimatedDialog("Uninstall failed", err.Error(), true)
					return
				}
				uninstallBtn.Disable()
				if mustExit {
					fyne.CurrentApp().Quit()
					return
				}
				g.showAnimatedDialog("Uninstalled", "The per-user installation was removed. Your preferences were kept.", false)
			}, win)
		}
		installerVBox.Add(container.NewHBox(installBtn, uninstallBtn))
	}

	if runtime.GOOS == "windows" {
		assocCheck := widget.NewCheck("Register .uproject association (Windows default-app choice still applies)", func(b bool) {
			appSettings.UprojectAssociated = b
		})
		assocCheck.SetChecked(appSettings.UprojectAssociated)
		installerVBox.Add(assocCheck)
		chooseDefault := widget.NewButton("Choose the .uproject default in Windows Settings", func() {
			target, _ := url.Parse("ms-settings:defaultapps")
			if err := fyne.CurrentApp().OpenURL(target); err != nil {
				appendStatus("Cannot open Windows Default Apps: %v", err)
			}
		})
		installerVBox.Add(chooseDefault)
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

	// Legacy preference is ignored: production connections always require TLS.
	appSettings.ForceHTTP = false
	forceHttpCheck := widget.NewCheck("HTTPS is required", nil)
	forceHttpCheck.SetChecked(true)
	forceHttpCheck.Disable()
	forceHttpCheck.Hide()

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
		g.showAnimatedDialog("Developer Mode", "Enabling Developer Mode reveals the Publisher Menu. API connections require HTTPS and separate network consent.\n\nWhen enabled on a local codebase, Installer Source Updates are disabled to prevent overwriting your local edits.", false)
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
			// Prevent double-v prefix (API already returns e.g. "vDev" or "v1.2.0")
			displayVer := newVer
			if !strings.HasPrefix(strings.ToLower(displayVer), "v") {
				displayVer = "v" + displayVer
			}
			updateBtn := newAccentButton("Update to "+displayVer, accentUpdate, func() {
				if g.win == nil {
					return
				}
				binPath := settings.NativeExecutablePath(appSettings)
				if !appSettings.InstalledNatively {
					executable, _ := os.Executable()
					workingDirectory, _ := os.Getwd()
					var err error
					binPath, err = updater.SourceDirectory(executable, workingDirectory)
					if err != nil {
						g.showAnimatedDialog("Update Error", err.Error(), true)
						return
					}
				}
				iconRes := loadIconResource()
				setStatus := g.playUpdateSequence(g.win, iconRes)
				setStatus("Downloading " + displayVer + "...")

				go func() {
					setStatus("Applying update...")
					err := updater.PerformUpdate(binPath, appSettings.InstalledNatively)
					if err == nil {
						setStatus("Restarting...")
						time.Sleep(500 * time.Millisecond)
						// For source-mode updates the new files are already in place;
						// relaunch ourselves before exiting.
						if !appSettings.InstalledNatively {
							if exe, e := os.Executable(); e == nil {
								exec.Command(exe).Start() //nolint:errcheck
							}
						}
						os.Exit(0)
					} else {
						// Write error file and restore UI
						if errPath, pathErr := settings.ErrorFilePath(); pathErr == nil {
							os.WriteFile(errPath, []byte(err.Error()), 0644) //nolint:errcheck
						}
						appendStatus("Update failed: %v", err)
					}
				}()
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
		networkSection,
		updateSection,
		pathsSection,
		installerSection,
		devSection,
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(200, 50), saveBtn)),
	)

	return container.NewPadded(container.NewScroll(content))
}
