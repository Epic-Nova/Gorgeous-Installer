package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxn/walk"

	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/unreal"
)

// GUIApp represents the native Windows GUI application
type GUIApp struct {
	config *config.Config
}

// NewGUIApp creates a new GUI app instance
func NewGUIApp(cfg *config.Config) *GUIApp {
	return &GUIApp{
		config: cfg,
	}
}

// Run starts the native Windows GUI
func (g *GUIApp) Run() {
	// Try to create the main window
	// Note: Walk library has known issues with tooltip initialization on some systems
	// If GUI fails, the app gracefully handles it
	
	defer func() {
		if r := recover(); r != nil {
			// Silently recover from GUI initialization panics
			// The user can use CLI mode with -cli flag
			fmt.Fprintf(os.Stderr, "GUI initialization failed: %v\nUse -cli flag for command-line mode\n", r)
		}
	}()

	mw, err := walk.NewMainWindow()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create main window: %v\n", err)
		return
	}
	defer mw.Dispose()

	mw.SetTitle("Gorgeous Installer")
	mw.SetSize(walk.Size{Width: 700, Height: 600})

	// Set layout on the parent window
	vlayout := walk.NewVBoxLayout()
	mw.SetLayout(vlayout)
	vlayout.SetMargins(walk.Margins{10, 10, 10, 10})
	vlayout.SetSpacing(10)

	// Title
	titleLabel, _ := walk.NewLabel(mw)
	titleLabel.SetText("GORGEOUS CORE - Installer")

	// Project section
	projectLabel, _ := walk.NewLabel(mw)
	projectLabel.SetText("Project: None selected")

	browseBtn, _ := walk.NewPushButton(mw)
	browseBtn.SetText("Browse for Project")

	// Version section
	versionLabel, _ := walk.NewLabel(mw)
	versionLabel.SetText("Engine Version: Not detected")

	// Status section  
	statusText, _ := walk.NewTextEdit(mw)
	statusText.SetText("Ready to install\n")
	statusText.SetReadOnly(true)
	statusText.SetMinMaxSize(walk.Size{Width: 0, Height: 100}, walk.Size{Width: 0, Height: 200})

	// Buttons
	installBtn, _ := walk.NewPushButton(mw)
	installBtn.SetText("Install")

	exitBtn, _ := walk.NewPushButton(mw)
	exitBtn.SetText("Exit")

	// Wire up events
	var projectPath string

	browseBtn.Clicked().Attach(func() {
		dlg := walk.FileDialog{
			Title:    "Select .uproject file",
			Filter:   "Unreal Project (*.uproject)|*.uproject",
			FilePath: projectPath,
		}
		if ok, _ := dlg.ShowOpen(mw); ok {
			projectPath = dlg.FilePath
			projectLabel.SetText(fmt.Sprintf("Project: %s", filepath.Base(projectPath)))
			
			// Detect version
			if version, _, err := unreal.GetEngineVersionFromProject(projectPath); err == nil {
				versionLabel.SetText(fmt.Sprintf("Engine Version: UE %s", version))
			} else {
				versionLabel.SetText(fmt.Sprintf("Engine Version: Error - %v", err))
			}
		}
	})

	installBtn.Clicked().Attach(func() {
		if projectPath == "" {
			walk.MsgBox(mw, "Error", "Please select a project first", walk.MsgBoxIconError)
			return
		}
		g.performInstall(projectPath, statusText, mw)
	})

	exitBtn.Clicked().Attach(func() {
		mw.Close()
	})

	// Show window and start event loop
	mw.SetVisible(true)
	mw.Run()
}

func (g *GUIApp) detectVersion(projectPath string, label *walk.Label) {
	if projectPath == "" {
		label.SetText("No project selected")
		return
	}

	version, _, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		label.SetText(fmt.Sprintf("Error: %v", err))
		return
	}

	label.SetText(fmt.Sprintf("UE %s", version))
}

func (g *GUIApp) performInstall(projectPath string, status *walk.TextEdit, mw *walk.MainWindow) {
	if projectPath == "" {
		walk.MsgBox(mw, "Error", "Please select a project first", walk.MsgBoxIconError)
		return
	}

	status.SetText("Starting installation...\n")

	_, enginePath, err := unreal.GetEngineVersionFromProject(projectPath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to detect engine: %v", err)
		status.AppendText(fmt.Sprintf("Error: %s\n", errMsg))
		walk.MsgBox(mw, "Error", errMsg, walk.MsgBoxIconError)
		return
	}

	status.AppendText(fmt.Sprintf("Engine: %s\n", enginePath))

	pluginPath, err := unreal.LocateGorgeousPlugin(filepath.Dir(projectPath), enginePath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to locate plugin: %v", err)
		status.AppendText(fmt.Sprintf("Error: %s\n", errMsg))
		walk.MsgBox(mw, "Error", errMsg, walk.MsgBoxIconError)
		return
	}

	status.AppendText(fmt.Sprintf("Plugin: %s\n", pluginPath))

	// Find first available pack
	if len(g.config.AvailableVersions) == 0 {
		errMsg := "No packs available"
		status.AppendText(fmt.Sprintf("Error: %s\n", errMsg))
		walk.MsgBox(mw, "Error", errMsg, walk.MsgBoxIconError)
		return
	}

	selectedPack := &g.config.AvailableVersions[0]
	status.AppendText(fmt.Sprintf("Installing %s %s...\n", g.config.PackType, selectedPack.Version))

	inst := installer.NewInstaller(pluginPath, g.config.PackType, selectedPack)
	if err := inst.Install(); err != nil {
		errMsg := fmt.Sprintf("Installation failed: %v", err)
		status.AppendText(fmt.Sprintf("Error: %s\n", errMsg))
		walk.MsgBox(mw, "Error", errMsg, walk.MsgBoxIconError)
		return
	}

	status.AppendText("Installation completed successfully!\n")
	walk.MsgBox(mw, "Success", "Pack installed successfully!", walk.MsgBoxIconInformation)
}
