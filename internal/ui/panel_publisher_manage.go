package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gorgeous-installer/internal/api"
	
)

func (g *GUIApp) buildManageSection(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	listVBox := container.NewVBox()

	refreshList := func() {
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
						g.showEditDialog(s, win, appendStatus)
					})
					delBtn := newAccentButton("Delete", accentUpdate, func() {
						dialog.ShowConfirm("Confirm Delete", "Are you sure you want to delete "+s.SystemId+"?", func(b bool) {
							if b {
								g.deleteSystem(win, s.SystemId, appendStatus)
							}
						}, win)
					})
					// delBtn.Importance = widget.DangerImportance

					lbl := widget.NewLabel(fmt.Sprintf("%s (%s) - v%s", s.DisplayName, s.SystemId, s.Version))
					lbl.TextStyle = fyne.TextStyle{Bold: true}

					row := container.NewHBox(lbl, layout.NewSpacer(), editBtn, delBtn)
					listVBox.Add(newGTCard("", "", row))
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

func (g *GUIApp) showEditDialog(sys api.SystemItem, win fyne.Window, appendStatus func(string, ...any)) {
	targetPluginEntry := widget.NewEntry()
	targetPluginEntry.SetText(sys.TargetPluginName)

	displayNameEntry := widget.NewEntry()
	displayNameEntry.SetText(sys.DisplayName)

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(sys.Description)

	isCoreCheck := widget.NewCheck("Is Core System?", nil)
	isCoreCheck.SetChecked(sys.IsCoreSystem)

	formItems := []*widget.FormItem{
		widget.NewFormItem("System ID (Locked)", widget.NewLabel(sys.SystemId)),
		widget.NewFormItem("Target Plugin", targetPluginEntry),
		widget.NewFormItem("Display Name", displayNameEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("", isCoreCheck),
	}

	dialog.ShowForm("Edit System "+sys.SystemId, "Save", "Cancel", formItems, func(b bool) {
		if b {
			regData := api.SystemRegistrationData{
				TargetPluginName: targetPluginEntry.Text,
				DisplayName:      displayNameEntry.Text,
				Description:      descEntry.Text,
				IsCoreSystem:     isCoreCheck.Checked,
			}
			g.patchSystem(win, sys.SystemId, regData, appendStatus)
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

func (g *GUIApp) deleteSystem(win fyne.Window, systemId string, appendStatus func(string, ...any)) {
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
	}()
}
