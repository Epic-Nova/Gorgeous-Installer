package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"gorgeous-installer/internal/api"
)

// CheckForUpdates fetches the latest installer version from the API.
func CheckForUpdates(currentVersion string) (string, bool) {
	resp, err := api.CheckInstallerUpdate()
	if err != nil {
		return "", false
	}
	if resp.UpdateAvailable && resp.LatestVersion != currentVersion {
		return resp.LatestVersion, true
	}
	return "", false
}

// PerformUpdate downloads the new payload from the API and runs a hidden shell script to overwrite the binary.
func PerformUpdate(binPath string) error {
	// 1. Get latest update info to retrieve download URL
	resp, err := api.CheckInstallerUpdate()
	if err != nil || !resp.UpdateAvailable {
		return fmt.Errorf("could not retrieve update info: %v", err)
	}

	payloadPath := filepath.Join(os.TempDir(), "gorgeous-installer-update-payload")
	
	// Download binary from the redirected S3 URL
	out, err := os.Create(payloadPath)
	if err != nil {
		return err
	}
	
	httpResp, err := http.Get(resp.DownloadUrl)
	if err != nil {
		out.Close()
		return err
	}
	defer httpResp.Body.Close()
	
	if httpResp.StatusCode >= 400 {
		out.Close()
		return fmt.Errorf("failed to download update: %s", httpResp.Status)
	}
	
	if _, err = io.Copy(out, httpResp.Body); err != nil {
		out.Close()
		return err
	}
	out.Close()
	os.Chmod(payloadPath, 0755)

	// 2. Create update script
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(home, ".local", "share", "gorgeous-update.sh")
	
	scriptContent := fmt.Sprintf(`#!/bin/bash
sleep 2
cp "%s" "%s"
chmod +x "%s"
rm "%s"
rm "$0"
`, payloadPath, binPath, binPath, payloadPath)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return err
	}

	// 3. Launch script in background
	cmd := exec.Command("bash", scriptPath)
	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}
