//go:build windows

package settings

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os"
)

const installerProgID = "GorgeousInstaller.ProjectFile"

// UserChoice is deliberately untouched. Modern Windows users choose their
// effective default through Windows Settings; this updates the per-user base
// association only after an explicit choice in the installer.
func UpdateRegistryAssociation(settings *AppSettings) error {
	return updateRegistryAssociationAt(settings, "Software\\Classes")
}
func registryDefault(path string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue("")
	if err == registry.ErrNotExist {
		return "", nil
	}
	return value, err
}
func setRegistryDefault(path, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("", value)
}
func updateRegistryAssociationAt(settings *AppSettings, classesRoot string) error {
	if settings == nil {
		return fmt.Errorf("installer settings are unavailable")
	}
	path := classesRoot + "\\.uproject"
	current, err := registryDefault(path)
	if err != nil {
		return fmt.Errorf("read current file association: %w", err)
	}
	if settings.UprojectAssociated {
		executable := NativeExecutablePath(settings)
		if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("install Gorgeous Installer for this user before registering its file association")
		}
		if settings.PrevUprojectCommand == "" {
			settings.PrevUprojectCommand = current
			if current == "" || current == installerProgID {
				settings.PrevUprojectCommand = "Unreal.ProjectFile"
			}
		}
		if err := setRegistryDefault(classesRoot+"\\"+installerProgID+"\\shell\\open\\command", "\""+executable+"\" \"%1\""); err != nil {
			return err
		}
		return setRegistryDefault(path, installerProgID)
	}
	if settings.PrevUprojectCommand != "" {
		// Another application or the user may have changed it since our opt-in.
		if current == installerProgID {
			if err := setRegistryDefault(path, settings.PrevUprojectCommand); err != nil {
				return err
			}
		}
		settings.PrevUprojectCommand = ""
	}
	return nil
}
