package settings

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeExecutablePathUsesConfiguredDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "custom install")
	settings := DefaultSettings()
	settings.LocalBinPath = directory
	expected := "gorgeous-installer"
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}
	if got := NativeExecutablePath(settings); got != filepath.Join(directory, expected) {
		t.Fatalf("native executable=%q", got)
	}
}
func TestNativeDetectionHonorsCustomDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "custom install")
	path := filepath.Join(directory, "gorgeous-installer.exe")
	if !nativeExecutableMatches(path, directory, "windows") {
		t.Fatal("custom Windows install was not detected")
	}
	if nativeExecutableMatches(path, filepath.Join(directory, "other"), "windows") {
		t.Fatal("portable executable misidentified as installed")
	}
	if nativeExecutableMatches(path, "", "windows") {
		t.Fatal("empty install directory accepted")
	}
}
