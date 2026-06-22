package updater

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gorgeous-installer/internal/api"
)

// CheckForUpdates fetches the latest installer version from the API.
func CheckForUpdates(currentVersion string, installedNatively bool) (string, bool) {
	updateType := "source"
	if installedNatively {
		updateType = "bin"
	}
	resp, err := api.CheckInstallerUpdate(updateType)
	if err != nil {
		return "", false
	}
	if resp.UpdateAvailable {
		// Compare versions as integers (e.g., 1.0.0 -> 100)
		latestInt := ParseVersion(resp.LatestVersion)
		currentInt := ParseVersion(currentVersion)
		
		if latestInt > currentInt {
			return resp.LatestVersion, true
		} else if latestInt == 0 && currentInt == 0 && resp.LatestVersion != currentVersion {
			// Fallback: If we couldn't parse either, but they differ
			return resp.LatestVersion, true
		}
	}
	return "", false
}

// ParseVersion strips dots and non-numeric characters to form an integer version
// e.g. "1.0.0" -> 100, "v1.2.3" -> 123
func ParseVersion(v string) int {
	v = strings.TrimPrefix(v, "v")
	clean := strings.ReplaceAll(v, ".", "")
	var result int
	fmt.Sscanf(clean, "%d", &result)
	return result
}

// PerformUpdate downloads the new payload ZIP and applies it based on installation type.
func PerformUpdate(binPath string, installedNatively bool) error {
	updateType := "source"
	if installedNatively {
		updateType = "bin"
	}

	resp, err := api.CheckInstallerUpdate(updateType)
	if err != nil || !resp.UpdateAvailable {
		return fmt.Errorf("could not retrieve update info: %v", err)
	}

	zipPath := filepath.Join(os.TempDir(), "gorgeous-installer-update.zip")
	
	// Download ZIP payload
	out, err := os.Create(zipPath)
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

	if installedNatively {
		return applyBinaryUpdate(zipPath, binPath)
	}
	return applySourceUpdate(zipPath, binPath)
}

func applySourceUpdate(zipPath string, sourceDir string) error {
	// SourceDir is the directory containing the source code (.go files)
	// We extract the ZIP directly into sourceDir, overwriting files.
	// Clean up zip after
	defer os.Remove(zipPath)
	return extractZip(zipPath, sourceDir)
}

func applyBinaryUpdate(zipPath string, binPath string) error {
	// We extract the ZIP (which contains the contents of the /build dir) to a temp dir.
	// Then we locate the correct executable for the OS and replace binPath.
	tempExtractDir := filepath.Join(os.TempDir(), "gorgeous-installer-extracted")
	os.RemoveAll(tempExtractDir)
	if err := os.MkdirAll(tempExtractDir, 0755); err != nil {
		return err
	}

	if err := extractZip(zipPath, tempExtractDir); err != nil {
		return err
	}

	// Locate the executable
	targetExeName := "gorgeous-installer"
	if runtime.GOOS == "windows" {
		targetExeName = "gorgeous-installer.exe"
	}

	extractedExe := filepath.Join(tempExtractDir, targetExeName)
	if _, err := os.Stat(extractedExe); err != nil {
		return fmt.Errorf("could not find executable %s in update zip", targetExeName)
	}
	os.Chmod(extractedExe, 0755)

	// Create update script to swap running binary
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	
	var scriptPath string
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(home, "gorgeous-update.bat")
		scriptContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
copy /y "%s" "%s"
del "%s"
del "%%~f0"
`, extractedExe, binPath, zipPath)
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return err
		}
		cmd := exec.Command("cmd", "/c", scriptPath)
		return cmd.Start()
	} else {
		// Ensure directory exists
		shareDir := filepath.Join(home, ".local", "share")
		os.MkdirAll(shareDir, 0755)
		
		scriptPath = filepath.Join(shareDir, "gorgeous-update.sh")
		scriptContent := fmt.Sprintf(`#!/bin/bash
sleep 2
cp "%s" "%s"
chmod +x "%s"
rm "%s"
rm -rf "%s"
rm "$0"
`, extractedExe, binPath, binPath, zipPath, tempExtractDir)

		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return err
		}
		cmd := exec.Command("bash", scriptPath)
		return cmd.Start()
	}
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue // Zip slip protection
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
