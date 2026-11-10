package settings

import (
	"fmt"
	"gorgeous-installer/internal/selfinstall"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NativeExecutablePath honors the configured directory and platform extension.
func NativeExecutablePath(appSettings *AppSettings) string {
	if appSettings == nil {
		appSettings = DefaultSettings()
	}
	name := "gorgeous-installer"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(appSettings.LocalBinPath, name)
}
func isInstalledNativelyAt(directory string) bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return false
	}
	return nativeExecutableMatches(executable, directory, runtime.GOOS)
}
func nativeExecutableMatches(executable, directory, platform string) bool {
	name := "gorgeous-installer"
	if platform == "windows" {
		name += ".exe"
	}
	expected := filepath.Join(directory, name)
	if directory == "" {
		return false
	}
	if platform == "windows" {
		return strings.EqualFold(filepath.Clean(executable), filepath.Clean(expected))
	}
	return filepath.Clean(executable) == filepath.Clean(expected)
}
func NativeInstallationPresent(appSettings *AppSettings) bool {
	if runtime.GOOS != "windows" || appSettings == nil {
		return false
	}
	return selfinstall.IsInstalled(appSettings.LocalBinPath)
}
func InstallNative(appSettings *AppSettings, version string) error {
	if appSettings == nil {
		return fmt.Errorf("installer settings are unavailable")
	}
	if _, err := selfinstall.Install(selfinstall.Options{InstallDir: appSettings.LocalBinPath, Version: version}); err != nil {
		return err
	}
	appSettings.InstalledNatively = isInstalledNativelyAt(appSettings.LocalBinPath)
	return SaveSettings(appSettings)
}

// UninstallNative returns true when the running application must now exit.
func UninstallNative(appSettings *AppSettings) (bool, error) {
	if appSettings == nil {
		return false, fmt.Errorf("installer settings are unavailable")
	}
	deferred, err := selfinstall.Uninstall(appSettings.LocalBinPath)
	if err != nil {
		return false, err
	}
	appSettings.InstalledNatively = false
	appSettings.UprojectAssociated = false
	appSettings.PrevUprojectCommand = ""
	// The owned uninstaller restores associations and persists only its own
	// registration's preferences, after this process exits if necessary.
	return deferred, nil
}
