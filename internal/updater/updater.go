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
	"gorgeous-installer/internal/settings"
)

// CheckForUpdates fetches the latest installer version from the API.
func CheckForUpdates(currentVersion string, installedNatively bool) (string, bool) {
	updateType := "source"
	if installedNatively {
		updateType = "bin"
	}
	resp, err := api.CheckInstallerUpdate(updateType, currentVersion)
	if err != nil {
		return "", false
	}
	if !resp.UpdateAvailable {
		return "", false
	}

	// Normalise both version strings the same way before any comparison.
	cleanCurrent := strings.TrimPrefix(currentVersion, "v")
	cleanLatest := strings.TrimPrefix(resp.LatestVersion, "v")

	// If we're already on the latest version, no update needed.
	// This is the primary guard — catches exact matches including non-semver
	// names like "Dev" that both sides share after stripping the v prefix.
	if cleanCurrent == cleanLatest {
		return "", false
	}

	systemId := "GorgeousInstaller-Source"
	if installedNatively {
		systemId = "GorgeousInstaller-Bin"
	}

	// Check if the current version is even registered in the API.
	// If not, treat the installed version as older than the master update
	// so the user always has a known-good entry point to follow the chain.
	systems, err := api.GetSystems()
	if err == nil {
		for _, sys := range systems {
			if sys.SystemId != systemId {
				continue
			}
			for _, v := range sys.Versions {
				// Strip v prefix from stored version too before comparing.
				if strings.TrimPrefix(v.Version, "v") == cleanCurrent {
					// Version is registered — fall through to numeric comparison.
					goto doNumericCompare
				}
			}
			// System found but current version not in its list — treat as outdated.
			return resp.LatestVersion, true
		}
	}

doNumericCompare:
	latestInt := ParseVersion(resp.LatestVersion)
	currentInt := ParseVersion(currentVersion)

	if latestInt > currentInt {
		return resp.LatestVersion, true
	}
	// Both versions are non-numeric (e.g. both "Dev") but differ — already
	// handled by the exact-match guard above, so no extra fallback needed.
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

	resp, err := api.CheckInstallerUpdate(updateType, "")
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

	// Get the error log path from settings
	errFilePath, _ := settings.ErrorFilePath()
	if errFilePath == "" {
		errFilePath = filepath.Join(os.TempDir(), "gorgeous-update-error.txt")
	}

	// Get the currently-running executable so the script can relaunch it (or the new one)
	currentExe, err := os.Executable()
	if err != nil {
		currentExe = binPath
	}

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
2>"%s" copy /y "%s" "%s"
if %%errorlevel%% == 0 (
    del /f "%s" 2>nul
    start "" "%s"
) else (
    start "" "%s"
)
del "%s" 2>nul
rmdir /s /q "%s" 2>nul
del "%%~f0"
`,
			errFilePath,
			extractedExe, binPath,
			errFilePath,
			binPath,
			currentExe,
			zipPath,
			tempExtractDir,
		)
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
cp "%s" "%s" 2>"%s"
if [ $? -eq 0 ]; then
    rm -f "%s"
    chmod +x "%s"
    rm -f "%s"
    rm -rf "%s"
    rm -f "$0"
    nohup "%s" &>/dev/null &
else
    chmod +x "%s"
    rm -f "%s"
    rm -rf "%s"
    rm -f "$0"
    nohup "%s" &>/dev/null &
fi
`,
			extractedExe, binPath, errFilePath,
			errFilePath,
			binPath,
			zipPath,
			tempExtractDir,
			binPath,
			currentExe,
			zipPath,
			tempExtractDir,
			currentExe,
		)

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
