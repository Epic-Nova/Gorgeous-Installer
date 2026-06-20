//go:build !cgo

package ui

import (
	"fmt"
	"os"

	"gorgeous-installer/internal/config"
)

// GUIApp represents the GUI wrapper when CGO is disabled.
type GUIApp struct {
	config *config.Config
}

// NewGUIApp creates a new GUI app instance.
func NewGUIApp(cfg *config.Config, recompileOnly bool, waitPid int, reopenProject bool, autoBuildProject bool, verifyCompat bool) *GUIApp {
	return &GUIApp{config: cfg}
}

// Run prints a clear message when the GUI build cannot run without CGO.
func (g *GUIApp) Run() {
	_ = g.config
	fmt.Fprintln(os.Stderr, "GUI requires CGO-enabled build environment. Use build.ps1 or run with -cli mode.")
}
