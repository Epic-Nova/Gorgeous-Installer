//go:build windows

package settings

import (
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestWindowsAssociationRestoresOnlyItsOwnChoice(t *testing.T) {
	root := "Software\\EpicNova\\SettingsAssociationValidation\\" + strconv.FormatInt(time.Now().UnixNano(), 16)
	classes := root + "\\Classes"
	paths := []string{classes + "\\" + installerProgID + "\\shell\\open\\command", classes + "\\" + installerProgID + "\\shell\\open", classes + "\\" + installerProgID + "\\shell", classes + "\\" + installerProgID, classes + "\\.uproject", classes, root}
	t.Cleanup(func() {
		for _, path := range paths {
			_ = registry.DeleteKey(registry.CURRENT_USER, path)
		}
		_ = registry.DeleteKey(registry.CURRENT_USER, "Software\\EpicNova\\SettingsAssociationValidation")
	})
	directory := t.TempDir()
	settings := DefaultSettings()
	settings.LocalBinPath = directory
	if err := os.WriteFile(filepath.Join(directory, "gorgeous-installer.exe"), []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	previous := "Previous.Project File With Spaces"
	if err := setRegistryDefault(classes+"\\.uproject", previous); err != nil {
		t.Fatal(err)
	}
	settings.UprojectAssociated = true
	if err := updateRegistryAssociationAt(settings, classes); err != nil {
		t.Fatal(err)
	}
	if settings.PrevUprojectCommand != previous {
		t.Fatal("previous association was truncated")
	}
	settings.UprojectAssociated = false
	if err := updateRegistryAssociationAt(settings, classes); err != nil {
		t.Fatal(err)
	}
	if got, _ := registryDefault(classes + "\\.uproject"); got != previous {
		t.Fatalf("restore=%q", got)
	}
	settings.UprojectAssociated = true
	if err := updateRegistryAssociationAt(settings, classes); err != nil {
		t.Fatal(err)
	}
	if err := setRegistryDefault(classes+"\\.uproject", "Newer.Application"); err != nil {
		t.Fatal(err)
	}
	settings.UprojectAssociated = false
	if err := updateRegistryAssociationAt(settings, classes); err != nil {
		t.Fatal(err)
	}
	if got, _ := registryDefault(classes + "\\.uproject"); got != "Newer.Application" {
		t.Fatal("a newer user/application choice was overwritten")
	}
}
