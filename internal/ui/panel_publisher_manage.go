package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"strings"

	"gorgeous-installer/internal/api"
)

func (g *GUIApp) buildManageContentSection(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	packsList := container.NewVBox()
	installerList := container.NewVBox()
	pluginsList := container.NewVBox()

	var refreshAll func()
	var refreshPacks func()
	var refreshInstaller func()
	var refreshPlugins func()

	refreshAll = func() {
		refreshPacks()
		refreshInstaller()
		refreshPlugins()
	}

	refreshPacks = func() {
		packsList.Objects = []fyne.CanvasObject{widget.NewLabel("Loading...")}
		packsList.Refresh()

		go func() {
			systems, err := api.GetAllSystems()
			fyne.Do(func() {
				packsList.Objects = nil
				if err != nil {
					packsList.Add(widget.NewLabel("Failed to fetch systems: " + err.Error()))
					packsList.Refresh()
					return
				}

				packs := filterPacks(systems)
				if len(packs) == 0 {
					packsList.Add(widget.NewLabel("No pack updates found."))
					packsList.Refresh()
					return
				}

				for _, sys := range packs {
					s := sys
					editBtn := widget.NewButtonWithIcon("Edit", theme.SettingsIcon(), func() {
						g.showEditDialog(s, win, refreshAll, appendStatus)
					})
					delBtn := newAccentButton("Delete", accentUpdate, func() {
						g.showDeleteDialog(s, win, refreshAll, appendStatus)
					})

					lbl := widget.NewLabel(fmt.Sprintf("%s (%s) - %s", s.DisplayName, s.SystemId, s.Version))
					lbl.TextStyle = fyne.TextStyle{Bold: true}
					lbl.Truncation = fyne.TextTruncateEllipsis

					row := container.NewBorder(nil, nil, nil, container.NewHBox(editBtn, delBtn), lbl)
					paddedRow := container.NewPadded(row)

					var childElements []fyne.CanvasObject
					for _, v := range s.Versions {
						masterText := ""
						if v.IsMasterUpdate {
							masterText = " (Master)"
						}
						childLbl := widget.NewLabel("  ↳ Update " + v.Version + masterText)
						if v.IsMasterUpdate {
							childLbl.TextStyle = fyne.TextStyle{Italic: true, Bold: true}
						} else {
							childLbl.TextStyle = fyne.TextStyle{Italic: true}
						}
						childElements = append(childElements, container.NewPadded(childLbl))
					}

					childrenBox := container.NewVBox(childElements...)

					bg := canvas.NewRectangle(theme.InputBackgroundColor())
					bg.CornerRadius = 4

					systemBox := container.NewStack(bg, container.NewVBox(paddedRow, childrenBox))
					packsList.Add(systemBox)
				}
				packsList.Refresh()
			})
		}()
	}

	refreshInstaller = func() {
		installerList.Objects = []fyne.CanvasObject{widget.NewLabel("Loading...")}
		installerList.Refresh()

		go func() {
			systems, err := api.GetAllSystems()
			fyne.Do(func() {
				installerList.Objects = nil
				if err != nil {
					installerList.Add(widget.NewLabel("Failed to fetch systems: " + err.Error()))
					installerList.Refresh()
					return
				}

				installerSystems := filterInstallerUpdates(systems)
				if len(installerSystems) == 0 {
					installerList.Add(widget.NewLabel("No installer updates found."))
					installerList.Refresh()
					return
				}

				for _, sys := range installerSystems {
					s := sys
					editBtn := widget.NewButtonWithIcon("Edit", theme.SettingsIcon(), func() {
						g.showEditDialog(s, win, refreshAll, appendStatus)
					})
					delBtn := newAccentButton("Delete", accentUpdate, func() {
						g.showDeleteDialog(s, win, refreshAll, appendStatus)
					})

					lbl := widget.NewLabel(fmt.Sprintf("%s (%s) - %s", s.DisplayName, s.SystemId, s.Version))
					lbl.TextStyle = fyne.TextStyle{Bold: true}
					lbl.Truncation = fyne.TextTruncateEllipsis

					row := container.NewBorder(nil, nil, nil, container.NewHBox(editBtn, delBtn), lbl)
					paddedRow := container.NewPadded(row)

					var childElements []fyne.CanvasObject
					for _, v := range s.Versions {
						masterText := ""
						if v.IsMasterUpdate {
							masterText = " (Master)"
						}
						childLbl := widget.NewLabel("  ↳ Update " + v.Version + masterText)
						if v.IsMasterUpdate {
							childLbl.TextStyle = fyne.TextStyle{Italic: true, Bold: true}
						} else {
							childLbl.TextStyle = fyne.TextStyle{Italic: true}
						}
						childElements = append(childElements, container.NewPadded(childLbl))
					}

					childrenBox := container.NewVBox(childElements...)

					bg := canvas.NewRectangle(theme.InputBackgroundColor())
					bg.CornerRadius = 4

					systemBox := container.NewStack(bg, container.NewVBox(paddedRow, childrenBox))
					installerList.Add(systemBox)
				}
				installerList.Refresh()
			})
		}()
	}

	refreshPlugins = func() {
		pluginsList.Objects = []fyne.CanvasObject{widget.NewLabel("Loading...")}
		pluginsList.Refresh()

		go func() {
			systems, err := api.GetAllSystems()
			fyne.Do(func() {
				pluginsList.Objects = nil
				if err != nil {
					pluginsList.Add(widget.NewLabel("Failed to fetch systems: " + err.Error()))
					pluginsList.Refresh()
					return
				}

				pluginSystems := filterPluginUpdates(systems)
				if len(pluginSystems) == 0 {
					pluginsList.Add(widget.NewLabel("No plugin updates found."))
					pluginsList.Refresh()
					return
				}

				for _, sys := range pluginSystems {
					s := sys
					editBtn := widget.NewButtonWithIcon("Edit", theme.SettingsIcon(), func() {
						g.showEditDialog(s, win, refreshAll, appendStatus)
					})
					delBtn := newAccentButton("Delete", accentUpdate, func() {
						g.showDeleteDialog(s, win, refreshAll, appendStatus)
					})

					lbl := widget.NewLabel(fmt.Sprintf("%s (%s) - %s", s.DisplayName, s.SystemId, s.Version))
					lbl.TextStyle = fyne.TextStyle{Bold: true}
					lbl.Truncation = fyne.TextTruncateEllipsis

					row := container.NewBorder(nil, nil, nil, container.NewHBox(editBtn, delBtn), lbl)
					paddedRow := container.NewPadded(row)

					var childElements []fyne.CanvasObject
					for _, v := range s.Versions {
						masterText := ""
						if v.IsMasterUpdate {
							masterText = " (Master)"
						}
						childLbl := widget.NewLabel("  ↳ Update " + v.Version + masterText)
						if v.IsMasterUpdate {
							childLbl.TextStyle = fyne.TextStyle{Italic: true, Bold: true}
						} else {
							childLbl.TextStyle = fyne.TextStyle{Italic: true}
						}
						childElements = append(childElements, container.NewPadded(childLbl))
					}

					childrenBox := container.NewVBox(childElements...)

					bg := canvas.NewRectangle(theme.InputBackgroundColor())
					bg.CornerRadius = 4

					systemBox := container.NewStack(bg, container.NewVBox(paddedRow, childrenBox))
					pluginsList.Add(systemBox)
				}
				pluginsList.Refresh()
			})
		}()
	}

	refreshBtn := newAccentButton("Refresh", accentUpdate, func() {
		refreshAll()
	})

	header := container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(100, 35), refreshBtn))

	tabs := container.NewAppTabs(
		container.NewTabItem("Packs", packsList),
		container.NewTabItem("Installer", installerList),
		container.NewTabItem("Plugins", pluginsList),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	manageSection := newGTCard("Manage Content", "Manage and edit existing release metadata",
		container.NewBorder(header, nil, nil, nil, tabs),
	)

	refreshAll()

	return manageSection
}

func filterPacks(systems []api.SystemItem) []api.SystemItem {
	var result []api.SystemItem
	for _, s := range systems {
		if s.IsPackUpdate() {
			result = append(result, s)
		}
	}
	return result
}

func filterInstallerUpdates(systems []api.SystemItem) []api.SystemItem {
	var result []api.SystemItem
	for _, s := range systems {
		if s.IsInstallerUpdate() {
			result = append(result, s)
		}
	}
	return result
}

func filterPluginUpdates(systems []api.SystemItem) []api.SystemItem {
	var result []api.SystemItem
	for _, s := range systems {
		if s.IsPluginUpdate() {
			result = append(result, s)
		}
	}
	return result
}

func (g *GUIApp) showDeleteDialog(s api.SystemItem, win fyne.Window, refreshList func(), appendStatus func(string, ...any)) {
	var versionOptions []string
	for _, v := range s.Versions {
		versionOptions = append(versionOptions, v.Version)
	}

	var d dialog.Dialog
	content := container.NewVBox()

	if len(versionOptions) > 0 {
		verSelect := widget.NewSelect(versionOptions, nil)
		verSelect.SetSelected(versionOptions[0])

		delVerBtn := widget.NewButtonWithIcon("Delete Version", theme.DeleteIcon(), func() {
			d.Hide()
			dialog.ShowConfirm("Confirm", "Delete version "+verSelect.Selected+"?", func(b bool) {
				if b {
					g.deleteSystemVersion(win, s.SystemId, verSelect.Selected, refreshList, appendStatus)
				}
			}, win)
		})

		content.Add(widget.NewLabel("Select a specific version to delete:"))
		content.Add(container.NewHBox(verSelect, delVerBtn))
		content.Add(widget.NewLabelWithStyle("OR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	}

	delSysBtn := widget.NewButtonWithIcon("Delete ENTIRE System", theme.DeleteIcon(), func() {
		d.Hide()
		dialog.ShowConfirm("Confirm", "Delete system "+s.SystemId+"?", func(b bool) {
			if b {
				g.deleteSystem(win, s.SystemId, refreshList, appendStatus)
			}
		}, win)
	})
	delSysBtn.Importance = widget.DangerImportance

	content.Add(widget.NewLabel("Delete the ENTIRE system and all its versions:"))
	content.Add(delSysBtn)

	d = dialog.NewCustom("Delete System or Version", "Cancel", content, win)
	d.Show()
}

func (g *GUIApp) showEditDialog(sys api.SystemItem, win fyne.Window, refreshList func(), appendStatus func(string, ...any)) {
	targetPluginEntry := widget.NewEntry()
	targetPluginEntry.SetText(sys.TargetPluginName)

	displayNameEntry := widget.NewEntry()
	displayNameEntry.SetText(sys.DisplayName)

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(sys.Description)

	minCoreEntry := widget.NewEntry()
	minCoreEntry.SetText(sys.MinimumCoreVersion)
	minCoreEntry.SetPlaceHolder("e.g., 1.0.0")

	sourcePathsEntry := widget.NewMultiLineEntry()
	for _, p := range sys.SourcePaths {
		sourcePathsEntry.SetText(sourcePathsEntry.Text + p + "\n")
	}
	sourcePathsEntry.SetPlaceHolder("One path per line")

	contentPathsEntry := widget.NewMultiLineEntry()
	for _, p := range sys.ContentPaths {
		contentPathsEntry.SetText(contentPathsEntry.Text + p + "\n")
	}
	contentPathsEntry.SetPlaceHolder("One path per line")

	var versionOptions []string
	for _, v := range sys.Versions {
		versionOptions = append(versionOptions, v.Version)
	}
	if len(versionOptions) == 0 {
		versionOptions = append(versionOptions, "None")
	}

	versionSelector := widget.NewSelect(versionOptions, nil)
	versionSelector.SetSelected(versionOptions[0])
	promoteCheck := widget.NewCheck("Set as Master Update", nil)
	promoteCheck.SetChecked(true)

	isCoreCheck := widget.NewCheck("Is Core System", nil)
	isCoreCheck.SetChecked(sys.IsCoreSystem)

	formItems := []*widget.FormItem{
		widget.NewFormItem("System ID (Locked)", widget.NewLabel(sys.SystemId)),
		widget.NewFormItem("Display Name", displayNameEntry),
		widget.NewFormItem("Description", descEntry),
	}

	isInstaller := sys.IsInstallerUpdate()
	isPack := sys.IsPackUpdate()
	isPlugin := sys.IsPluginUpdate()

	if !isInstaller {
		formItems = append(formItems, widget.NewFormItem("Minimum Core Version", minCoreEntry))
	}

	if isPack {
		formItems = append(formItems,
			widget.NewFormItem("Source Paths", sourcePathsEntry),
			widget.NewFormItem("Content Paths", contentPathsEntry),
			widget.NewFormItem("Core Setup", isCoreCheck),
		)
	} else if isPlugin {
		formItems = append(formItems, widget.NewFormItem("Target Plugin Name", targetPluginEntry))
	}

	formItems = append(formItems, widget.NewFormItem("Promote Version", container.NewVBox(versionSelector, promoteCheck)))

	dialog.ShowForm("Edit System "+sys.SystemId, "Save", "Cancel", formItems, func(b bool) {
		if b {
			regData := api.SystemRegistrationData{
				DisplayName:        displayNameEntry.Text,
				Description:        descEntry.Text,
				IsCoreSystem:       isCoreCheck.Checked,
				MinimumCoreVersion: minCoreEntry.Text,
			}
			if isPlugin {
				regData.TargetPluginName = targetPluginEntry.Text
			}
			if isPack {
				regData.SourcePaths = parsePaths(sourcePathsEntry.Text)
				regData.ContentPaths = parsePaths(contentPathsEntry.Text)
			}

			doPromote := promoteCheck.Checked
			selectedVer := versionSelector.Selected

			go func() {
				g.patchSystem(win, sys.SystemId, regData, appendStatus)

				if doPromote && selectedVer != "None" {
					appendStatus("Promoting %s v%s to Master Update...", sys.SystemId, selectedVer)
					err := api.PromoteSystemVersion(sys.SystemId, selectedVer)
					if err != nil {
						appendStatus("Failed to promote update: %v", err)
					} else {
						appendStatus("Successfully promoted %s v%s", sys.SystemId, selectedVer)
					}
				}

				fyne.Do(func() {
					refreshList()
				})
			}()
		}
	}, win)
}

func parsePaths(text string) []string {
	var result []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (g *GUIApp) patchSystem(win fyne.Window, systemId string, regData api.SystemRegistrationData, appendStatus func(string, ...any)) {
	appendStatus("Fetching challenge for editing %s...", systemId)
	challenge, err := api.GetPublishChallenge(systemId)
	if err != nil {
		appendStatus("Error fetching challenge: %v", err)
		return
	}

	appendStatus("Please touch your YubiKey to sign the edit request...")
	sig, err := g.performPIVSign(win, challenge, appendStatus, nil)
	if err != nil {
		appendStatus("Failed to sign challenge: %v", err)
		return
	}

	err = api.PatchSystem(systemId, sig, regData)
	if err != nil {
		appendStatus("Failed to update system: %v", err)
		return
	}
	appendStatus("Successfully updated %s", systemId)
}

func (g *GUIApp) deleteSystem(win fyne.Window, systemId string, refreshList func(), appendStatus func(string, ...any)) {
	go func() {
		appendStatus("Fetching challenge for deleting %s...", systemId)
		challenge, err := api.GetPublishChallenge(systemId)
		if err != nil {
			appendStatus("Error fetching challenge: %v", err)
			return
		}

		appendStatus("Please touch your YubiKey to sign the delete request...")
		sig, err := g.performPIVSign(win, challenge, appendStatus, nil)
		if err != nil {
			appendStatus("Failed to sign challenge: %v", err)
			return
		}

		err = api.DeleteSystem(systemId, sig)
		if err != nil {
			appendStatus("Failed to delete system: %v", err)
			return
		}
		appendStatus("Successfully deleted %s", systemId)
		if refreshList != nil {
			refreshList()
		}
	}()
}

func (g *GUIApp) deleteSystemVersion(win fyne.Window, systemId, version string, refreshList func(), appendStatus func(string, ...any)) {
	go func() {
		appendStatus("Fetching challenge for deleting %s v%s...", systemId, version)
		challenge, err := api.GetPublishChallenge(systemId)
		if err != nil {
			appendStatus("Error fetching challenge: %v", err)
			return
		}

		appendStatus("Please touch your YubiKey to sign the version delete request...")
		sig, err := g.performPIVSign(win, challenge, appendStatus, nil)
		if err != nil {
			appendStatus("Failed to sign challenge: %v", err)
			return
		}

		err = api.DeleteSystemVersion(systemId, version, sig)
		if err != nil {
			appendStatus("Failed to delete version: %v", err)
			return
		}
		appendStatus("Successfully deleted %s v%s", systemId, version)
		if refreshList != nil {
			refreshList()
		}
	}()
}
