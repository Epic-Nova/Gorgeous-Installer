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

	"gorgeous-installer/internal/api"
)

func (g *GUIApp) buildManageSection(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	listVBox := container.NewVBox()

	var refreshList func()
	refreshList = func() {
		listVBox.Objects = []fyne.CanvasObject{widget.NewLabel("Loading...")}
		listVBox.Refresh()

		go func() {
			systems, err := api.GetSystems()
			
			fyne.Do(func() {
				listVBox.Objects = nil
				if err != nil {
					listVBox.Add(widget.NewLabel("Failed to fetch systems: " + err.Error()))
					listVBox.Refresh()
					return
				}

				if len(systems) == 0 {
					listVBox.Add(widget.NewLabel("No packs found."))
					listVBox.Refresh()
					return
				}

				// Move Installer Updates to the top if present
				// Assuming systemId == "GorgeousInstaller"
				var sorted []api.SystemItem
				for _, s := range systems {
					if s.SystemId == "GorgeousInstaller" {
						sorted = append(sorted, s)
					}
				}
				for _, s := range systems {
					if s.SystemId != "GorgeousInstaller" {
						sorted = append(sorted, s)
					}
				}

				for _, sys := range sorted {
					s := sys
					editBtn := widget.NewButtonWithIcon("Edit", theme.SettingsIcon(), func() {
						g.showEditDialog(s, win, refreshList, appendStatus)
					})
					delBtn := newAccentButton("Delete", accentUpdate, func() {
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
					})

					// UI Fixes for card row
					lbl := widget.NewLabel(fmt.Sprintf("%s (%s) - %s", s.DisplayName, s.SystemId, s.Version))
					lbl.TextStyle = fyne.TextStyle{Bold: true}
					lbl.Truncation = fyne.TextTruncateEllipsis

					row := container.NewBorder(nil, nil, nil, container.NewHBox(editBtn, delBtn), lbl)
					paddedRow := container.NewPadded(row)

					// Render Real Child Elements (Updates)
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

					// Custom background to avoid the empty GTCard padding issue
					bg := canvas.NewRectangle(theme.InputBackgroundColor())
					bg.CornerRadius = 4

					systemBox := container.NewStack(bg, container.NewVBox(paddedRow, childrenBox))

					listVBox.Add(systemBox)
				}
				listVBox.Refresh()
			})
		}()
	}

	refreshBtn := newAccentButton("Refresh", accentUpdate, func() {
		refreshList()
	})

	header := container.NewHBox(widget.NewLabelWithStyle("Registered Packs & Systems", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(100, 35), refreshBtn))

	manageSection := container.NewBorder(header, nil, nil, nil, container.NewVScroll(listVBox))
	
	// initial load
	refreshList()

	return manageSection
}

func (g *GUIApp) showEditDialog(sys api.SystemItem, win fyne.Window, refreshList func(), appendStatus func(string, ...any)) {
	targetPluginEntry := widget.NewEntry()
	targetPluginEntry.SetText(sys.TargetPluginName)

	displayNameEntry := widget.NewEntry()
	displayNameEntry.SetText(sys.DisplayName)

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(sys.Description)

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
		widget.NewFormItem("Target Plugin", targetPluginEntry),
		widget.NewFormItem("Display Name", displayNameEntry),
		widget.NewFormItem("Description", descEntry),
	}

	if sys.SystemId != "GorgeousInstaller" {
		formItems = append(formItems, widget.NewFormItem("Core Setup", isCoreCheck))
	}
	formItems = append(formItems, widget.NewFormItem("Promote Version", container.NewVBox(versionSelector, promoteCheck)))

	dialog.ShowForm("Edit System "+sys.SystemId, "Save", "Cancel", formItems, func(b bool) {
		if b {
			regData := api.SystemRegistrationData{
				TargetPluginName: targetPluginEntry.Text,
				DisplayName:      displayNameEntry.Text,
				Description:      descEntry.Text,
				IsCoreSystem:     isCoreCheck.Checked,
			}
			doPromote := promoteCheck.Checked
			selectedVer := versionSelector.Selected

			go func() {
				g.patchSystem(win, sys.SystemId, regData, appendStatus)
				
				if doPromote {
					appendStatus("Promoting %s v%s to Master Update...", sys.SystemId, selectedVer)
					err := api.PromoteSystemVersion(sys.SystemId, selectedVer)
					if err != nil {
						appendStatus("Failed to promote update: %v", err)
					} else {
						appendStatus("Successfully promoted %s v%s", sys.SystemId, selectedVer)
					}
				}

				// Auto-refresh the list after a successful edit
				fyne.Do(func() {
					refreshList()
				})
			}()
		}
	}, win)
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
	}()
}
