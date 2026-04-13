package ui

import (
	"gorgeous-installer/internal/config"
)

// GUIApp represents the GUI application
type GUIApp struct {
	config *config.Config
	webapp *WebApp
}

// NewGUIApp creates a new GUI application instance
func NewGUIApp(cfg *config.Config) *GUIApp {
	return &GUIApp{
		config: cfg,
		webapp: NewWebApp(cfg),
	}
}

// Run starts the GUI application (now web-based)
func (g *GUIApp) Run() {
	g.webapp.Run()
}
