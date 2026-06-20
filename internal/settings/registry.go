package settings

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// UpdateRegistryAssociation applies or reverts the .uproject Windows registry association.
func UpdateRegistryAssociation(appSettings *AppSettings) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	if appSettings.UprojectAssociated {
		// We want to associate
		// First, backup the current value if we haven't already
		if appSettings.PrevUprojectCommand == "" {
			cmd := exec.Command("reg", "query", `HKCU\Software\Classes\.uproject`, "/ve")
			out, err := cmd.Output()
			if err == nil {
				// Parse output to find the data
				// Example output:
				// HKEY_CURRENT_USER\Software\Classes\.uproject
				//     (Default)    REG_SZ    Unreal.ProjectFile
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					if strings.Contains(line, "REG_SZ") {
						parts := strings.Fields(line)
						if len(parts) >= 3 {
							appSettings.PrevUprojectCommand = parts[len(parts)-1]
						}
					}
				}
			}
			// If it's empty, default to Unreal.ProjectFile just in case
			if appSettings.PrevUprojectCommand == "" || appSettings.PrevUprojectCommand == "GorgeousInstaller.ProjectFile" {
				appSettings.PrevUprojectCommand = "Unreal.ProjectFile"
			}
		}

		// Apply the association
		err := exec.Command("reg", "add", `HKCU\Software\Classes\.uproject`, "/ve", "/d", "GorgeousInstaller.ProjectFile", "/f").Run()
		if err != nil {
			return fmt.Errorf("failed to write registry key: %v", err)
		}
	} else {
		// We want to revert
		if appSettings.PrevUprojectCommand != "" {
			err := exec.Command("reg", "add", `HKCU\Software\Classes\.uproject`, "/ve", "/d", appSettings.PrevUprojectCommand, "/f").Run()
			if err != nil {
				return fmt.Errorf("failed to restore registry key: %v", err)
			}
			appSettings.PrevUprojectCommand = "" // Clear it since we restored it
		}
	}

	return nil
}
