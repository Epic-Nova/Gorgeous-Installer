package ui

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/unreal"
)

// GUIApp represents the GUI application
type GUIApp struct {
	app    fyne.App
	config *config.Config
	window fyne.Window

	// UI state
	selectedProject binding.DataBinding
	selectedVersion binding.DataBinding
	packType        binding.DataBinding
}

// NewGUIApp creates a new GUI application instance
func NewGUIApp(cfg *config.Config) *GUIApp {
	return &GUIApp{
		config:          cfg,
		selectedProject: binding.NewString(),
		selectedVersion: binding.NewString(),
		packType:        binding.NewString(),
	}
}

// Run starts the GUI application
func (g *GUIApp) Run() {
	myApp := app.NewWithID("gorgeous-installer")
	g.app = myApp

	w := myApp.NewWindow()
	g.window = w
	w.SetTitle("Gorgeous Installer - " + g.config.PackName)
	w.Resize(fyne.NewSize(600, 700))

	// Build the UI
	content := g.buildMainUI()
	w.SetContent(content)

	w.ShowAndRun()
}

// buildMainUI creates the main user interface
func (g *GUIApp) buildMainUI() *fyne.Container {
	// Title
	title := widget.NewLabel("Gorgeous Unreal Engine Plugin Installer")
	titleRichText := widget.NewRichTextFromMarkdown("# " + "Gorgeous Unreal Engine Plugin Installer")

	subtitle := widget.NewLabel("Pack: " + g.config.PackName + " (" + g.config.PackType + ")")

	// Project selection section
	projectLabel := widget.NewLabel("Select Unreal Engine Project:")
	projectPathLabel := widget.NewLabel("No project selected")
	projectPathLabel.Alignment = fyne.TextAlignCenter

	selectProjectBtn := widget.NewButton("Browse for .uproject", func() {
		g.selectProjectFile(projectPathLabel)
	})

	// Engine version display
	versionLabel := widget.NewLabel("Engine Version:")
	detectedVersionLabel := widget.NewLabel("Not detected")
	detectedVersionLabel.Alignment = fyne.TextAlignCenter

	// Pack version selection
	packVersionLabel := widget.NewLabel("Select Pack Version:")
	
	versions := g.getAvailableVersions()
	packVersionSelect := widget.NewSelect(versions, func(s string) {
		if s != "" {
			g.selectedVersion.Set(s)
		}
	})
	packVersionSelect.PlaceHolder = "Choose a version"

	// Installation status
	statusLabel := widget.NewLabel("")
	statusLabel.Alignment = fyne.TextAlignCenter

	// Controls
	installBtn := widget.NewButton("Install Pack", func() {
		g.performInstallation(projectPathLabel, detectedVersionLabel, packVersionSelect, statusLabel)
	})
	installBtn.Importance = widget.HighImportance

	exitBtn := widget.NewButton("Exit", func() {
		g.window.Close()
	})

	// Layout
	content := container.NewVBox(
		titleRichText,
		widget.NewSeparator(),
		subtitle,
		widget.NewSeparator(),
		projectLabel,
		selectProjectBtn,
		projectPathLabel,
		widget.NewSeparator(),
		versionLabel,
		detectedVersionLabel,
		widget.NewSeparator(),
		packVersionLabel,
		packVersionSelect,
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		container.NewHBox(
			installBtn,
			exitBtn,
		),
	)

	return container.NewVBox(
		container.NewPadded(content),
	)
}

// selectProjectFile opens a file dialog to select a .uproject file
func (g *GUIApp) selectProjectFile(label *widget.Label) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(fmt.Errorf("error opening file: %w", err), g.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		// Get the file path
		filePath := reader.URI().Path()
		label.SetText(filepath.Base(filePath))
		g.selectedProject.Set(filePath)

		// Try to detect engine version
		g.detectEngineVersion(filePath)
	}, g.window)
}

// detectEngineVersion reads the .uproject file and extracts the engine version
func (g *GUIApp) detectEngineVersion(projectPath string) {
	version, _, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to detect engine version: %w", err), g.window)
		return
	}

	g.selectedVersion.Set(version)
}

// getAvailableVersions returns the list of available pack versions
func (g *GUIApp) getAvailableVersions() []string {
	var versions []string
	for _, pv := range g.config.AvailableVersions {
		versions = append(versions, pv.Version)
	}
	return versions
}

// performInstallation executes the installation process
func (g *GUIApp) performInstallation(
	projectLabel *widget.Label,
	versionLabel *widget.Label,
	versionSelect *widget.Select,
	statusLabel *widget.Label,
) {
	// Validate inputs
	projectPath := projectLabel.Text
	if projectPath == "" || projectPath == "No project selected" {
		dialog.ShowError(fmt.Errorf("please select a project"), g.window)
		return
	}

	selectedVer := versionSelect.Selected
	if selectedVer == "" {
		dialog.ShowError(fmt.Errorf("please select a pack version"), g.window)
		return
	}

	// Get selected pack version
	var selectedPack *config.PackVersion
	for i, pv := range g.config.AvailableVersions {
		if pv.Version == selectedVer {
			selectedPack = &g.config.AvailableVersions[i]
			break
		}
	}

	if selectedPack == nil {
		dialog.ShowError(fmt.Errorf("invalid pack version selected"), g.window)
		return
	}

	// Show progress dialog
	progressDialog := dialog.NewProgress("Installing...", "Starting installation...", g.window)
	progressDialog.Show()
	defer progressDialog.Hide()

	// Get engine path
	_, enginePath, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("failed to get engine path: %w", err), g.window)
		return
	}

	// Find plugin
	pluginPath, err := unreal.LocateGorgeousPlugin(filepath.Dir(projectPath), enginePath)
	if err != nil {
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("failed to locate plugin: %w", err), g.window)
		return
	}

	// Perform installation
	inst := installer.NewInstaller(pluginPath, g.config.PackType, selectedPack)
	if err := inst.Install(); err != nil {
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("installation failed: %w", err), g.window)
		return
	}

	progressDialog.Hide()
	statusLabel.SetText("✓ Installation completed successfully!")
	dialog.ShowInformation("Success", "Pack installed successfully!", g.window)
}
