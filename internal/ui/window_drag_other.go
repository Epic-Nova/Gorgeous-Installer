//go:build cgo && !windows

package ui

import "fyne.io/fyne/v2"

func startRoundedWindowStyling(_ fyne.Window, _ int32) {
	// No-op on non-Windows platforms.
}

func beginWindowDrag(_ string) bool {
	return false
}

func moveWindowByDelta(_ string, _ float32, _ float32) {
	// No-op on non-Windows platforms.
}
