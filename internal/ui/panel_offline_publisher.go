package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"gorgeous-installer/internal/config"
)

type versionEntry struct {
	ueVer      string
	sourcePath string
}

func (g *GUIApp) showOfflinePublisherDialog(win fyne.Window, publishMode string, manifest *SystemManifest, versions []versionEntry, installerPath string, sysVer string, appendStatus func(string, ...any)) {
	outDirEntry := widget.NewEntry()
	outDirEntry.SetPlaceHolder("Output Directory")
	
	browseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if lu != nil {
				outDirEntry.SetText(lu.Path())
			}
		}, win)
	})

	var titleText string
	if manifest != nil {
		titleText = fmt.Sprintf("Offline Builder for %s", manifest.Name)
	} else {
		titleText = "Offline Builder"
	}

	content := container.NewVBox(
		widget.NewLabel(titleText),
		widget.NewLabel("Select output directory for the standalone installer:"),
		widget.NewSeparator(),
		container.NewHBox(widget.NewLabel("Output:"), container.NewGridWrap(fyne.NewSize(300, 35), outDirEntry), browseBtn),
	)

	d := dialog.NewCustomConfirm("Offline Publishing", "Build Installer", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		if publishMode != "Installer Update" && len(versions) == 0 {
			dialog.ShowError(fmt.Errorf("please add at least one version mapping"), win)
			return
		}
		if outDirEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("please select an output directory"), win)
			return
		}

		go g.runOfflinePublish(win, publishMode, versions, installerPath, sysVer, outDirEntry.Text, manifest, appendStatus)
	}, win)
	d.Resize(fyne.NewSize(550, 200))
	d.Show()
}

func (g *GUIApp) runOfflinePublish(win fyne.Window, publishMode string, versions []versionEntry, installerPath string, sysVer string, outDir string, manifest *SystemManifest, appendStatus func(string, ...any)) {
	var manifestID string
	var manifestName string

	if manifest != nil {
		manifestID = manifest.ID
		manifestName = manifest.Name
	} else {
		// Read from the first version entry
		if len(versions) > 0 {
			manifestPath := filepath.Join(versions[0].sourcePath, "SystemManifest.json")
			manifestData, err := os.ReadFile(manifestPath)
			if err == nil {
				var localManifest SystemManifest
				if json.Unmarshal(manifestData, &localManifest) == nil {
					manifestID = localManifest.ID
					manifestName = localManifest.Name
				}
			}
		}
	}

	if manifestID == "" {
		manifestID = "OfflinePack"
	}
	if manifestName == "" {
		manifestName = "Offline Pack"
	}

	var progress *dialog.CustomDialog
	var statusLbl *widget.Label

	fyne.Do(func() {
		statusLbl = widget.NewLabel("Starting offline publish build for " + manifestName + "...")
		statusLbl.Wrapping = fyne.TextWrapWord
		progBar := widget.NewProgressBarInfinite()
		content := container.NewVBox(statusLbl, progBar)
		progress = dialog.NewCustom("Offline Publisher Progress", "Hide to Background", content, win)
		progress.Show()
	})

	updateStatus := func(msg string, args ...any) {
		text := fmt.Sprintf(msg, args...)
		appendStatus("%s", text)
		fyne.Do(func() {
			if statusLbl != nil {
				statusLbl.SetText(text)
			}
		})
	}

	updateStatus("Starting offline publish build for %s", manifestName)

	if publishMode == "Installer Update" {
		updateStatus("Zipping Installer Update to output directory...")
		outZip := filepath.Join(outDir, fmt.Sprintf("%s-%s.zip", manifestID, sysVer))
		
		var cmdZip *exec.Cmd
		if manifestID == "GorgeousInstaller-Source" {
			cmdZip = exec.Command("zip", "-r", outZip, ".", "-x", "build/*", "build", "*.exe", "*.syso", ".git/*", ".git", "*.log", "*.gti")
			cmdZip.Dir = installerPath
		} else {
			var buildDir string
			if filepath.Base(installerPath) == "build" {
				buildDir = installerPath
			} else {
				buildDir = filepath.Join(installerPath, "build")
				if _, err := os.Stat(buildDir); os.IsNotExist(err) {
					if _, errBin := os.Stat(filepath.Join(installerPath, "gorgeous-installer")); errBin == nil {
						buildDir = installerPath
					} else if _, errExe := os.Stat(filepath.Join(installerPath, "gorgeous-installer.exe")); errExe == nil {
						buildDir = installerPath
					} else {
						updateStatus("Build directory not found! Please compile binaries first.")
						fyne.Do(func() {
							if progress != nil {
								progress.Hide()
							}
						})
						return
					}
				}
			}
			cmdZip = exec.Command("zip", "-r", outZip, ".")
			cmdZip.Dir = buildDir
		}
		
		if err := cmdZip.Run(); err != nil {
			updateStatus("Failed to zip Installer Update: %v", err)
			fyne.Do(func() {
				if progress != nil {
					progress.Hide()
				}
			})
			return
		}
		
		fyne.Do(func() {
			if progress != nil {
				progress.Hide()
			}
			dialog.ShowInformation("Success", "Offline Installer Update package built at:\n"+outZip, win)
		})
		return
	}

	tempDir, err := os.MkdirTemp("", "gorgeous-offline-*")
	if err != nil {
		updateStatus("Failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	updateStatus("Cloning Gorgeous Installer repository...")
	cmdClone := exec.Command("git", "clone", "--depth", "1", "https://github.com/Epic-Nova/Gorgeous-Installer.git", tempDir)
	if err := cmdClone.Run(); err != nil {
		updateStatus("Git clone failed: %v", err)
		return
	}

	packsDir := filepath.Join(tempDir, "packs")
	os.MkdirAll(packsDir, 0755)

	var availVersions []config.PackVersion

	// Determine actualPluginName from first version's sourcePath
	firstPluginRoot := versions[0].sourcePath
	for firstPluginRoot != "" && firstPluginRoot != string(filepath.Separator) && firstPluginRoot != "." {
		matches, _ := filepath.Glob(filepath.Join(firstPluginRoot, "*.uplugin"))
		if len(matches) > 0 {
			break
		}
		parent := filepath.Dir(firstPluginRoot)
		if parent == firstPluginRoot {
			firstPluginRoot = versions[0].sourcePath
			break
		}
		firstPluginRoot = parent
	}
	actualPluginName := filepath.Base(firstPluginRoot)

	for _, v := range versions {
		updateStatus("Packaging payload for UE %s (Sys %s)...", v.ueVer, sysVer)
		var localManifest SystemManifest
		if publishMode == "Installer Update" {
			// For Installer updates, we don't need a manifest file on disk. We just use the provided one.
			localManifest = *manifest
			localManifest.PayloadPaths = []string{"."}
		} else if publishMode == "Plugin Update" {
			// For Plugin Updates, we also don't use SystemManifest.json. We use the whole directory.
			localManifest = *manifest
			localManifest.PayloadPaths = []string{"."}
		} else {
			manifestPath := filepath.Join(v.sourcePath, "SystemManifest.json")
			manifestData, err := os.ReadFile(manifestPath)
			if err != nil {
				updateStatus("Failed to read manifest for UE %s in %s: %v", v.ueVer, v.sourcePath, err)
				return
			}
			if err := json.Unmarshal(manifestData, &localManifest); err != nil {
				updateStatus("Failed to parse manifest for UE %s: %v", v.ueVer, err)
				return
			}
		}

		packName := fmt.Sprintf("%s-%s", manifestID, v.ueVer)
		packPath := filepath.Join(packsDir, packName)
		os.MkdirAll(packPath, 0755)

		vPluginRoot := v.sourcePath
		for vPluginRoot != "" && vPluginRoot != string(filepath.Separator) && vPluginRoot != "." {
			matches, _ := filepath.Glob(filepath.Join(vPluginRoot, "*.uplugin"))
			if len(matches) > 0 {
				break
			}
			parent := filepath.Dir(vPluginRoot)
			if parent == vPluginRoot {
				vPluginRoot = v.sourcePath
				break
			}
			vPluginRoot = parent
		}

		var pathsToCopy []string
		if len(localManifest.PayloadPaths) > 0 {
			for _, p := range localManifest.PayloadPaths {
				pathsToCopy = append(pathsToCopy, p)
			}
		} else {
			pathsToCopy = []string{"."}
		}

		var exclusions []string
		if publishMode == "Plugin Update" {
			filepath.Walk(vPluginRoot, func(p string, info os.FileInfo, err error) error {
				if err != nil { return nil }
				if info.IsDir() {
					base := info.Name()
					if base == ".git" || base == "Binaries" || base == "Intermediate" || base == "Saved" || base == "DerivedDataCache" || base == ".vs" {
						return filepath.SkipDir
					}
				}
				if !info.IsDir() && info.Name() == "SystemManifest.json" {
					if b, err := os.ReadFile(p); err == nil {
						var m SystemManifest
						if json.Unmarshal(b, &m) == nil {
							if !m.IsCoreSystem {
								exclusions = append(exclusions, m.PayloadPaths...)
							}
						}
					}
				}
				return nil
			})
		}

		for _, relPath := range pathsToCopy {
			src := filepath.Join(vPluginRoot, relPath)
			dst := filepath.Join(packPath, relPath)
			
			if info, err := os.Stat(src); err == nil {
				if info.IsDir() {
					os.MkdirAll(dst, 0755)
					if err := copyDirFiltered(src, dst, exclusions); err != nil {
						updateStatus("Copy dir failed for UE %s (src: %s): %v", v.ueVer, src, err)
						return
					}
				} else {
					os.MkdirAll(filepath.Dir(dst), 0755)
					if err := copyFile(src, dst); err != nil {
						updateStatus("Copy file failed for UE %s (src: %s): %v", v.ueVer, src, err)
						return
					}
				}
			} else {
				updateStatus("Warning: Path not found: %s. Skipping...", src)
				continue
			}
		}

		shaFilePath := filepath.Join(packsDir, fmt.Sprintf("%s.sha256", packName))
		if err := generateSHA256Manifest(packPath, shaFilePath); err != nil {
			updateStatus("Warning: failed to generate SHA manifest for %s: %v", packName, err)
		}

		availVersions = append(availVersions, config.PackVersion{
			Version: v.ueVer,
			Path:    fmt.Sprintf("packs/%s", packName),
			SHAFile: fmt.Sprintf("packs/%s.sha256", packName),
		})
	}

	updateStatus("Generating config.json...")
	cfg := config.Config{
		PackName:          manifestID,
		PackType:          "hybrid",
		PluginName:        actualPluginName,
		AvailableVersions: availVersions,
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tempDir, "config.json"), cfgData, 0644)

	// Installer Update bypass is handled at the beginning of the function

	updateStatus("Compiling gorgeous-installer binary via build.sh (Windows)...")
	outExe := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s.exe", manifestID))
	
	cmdBuild := exec.Command("bash", "./build.sh")
	cmdBuild.Dir = tempDir
	cmdBuild.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=1", "CC=x86_64-w64-mingw32-gcc", "CXX=x86_64-w64-mingw32-g++")
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		updateStatus("Windows build skipped or failed: %v\n%s\n(Note: Cross-compiling Fyne to Windows on Linux requires 'gcc-mingw-w64' installed)", err, string(out))
	} else {
		srcExe := filepath.Join(tempDir, "build", "gorgeous-installer.exe")
		if err := copyFile(srcExe, outExe); err != nil {
			updateStatus("Failed to copy Windows binary: %v", err)
		} else {
			updateStatus("Windows build successful!")
		}
	}

	updateStatus("Compiling gorgeous-installer binary via build.sh (Linux)...")
	outBin := filepath.Join(outDir, fmt.Sprintf("gorgeous-installer-%s", manifestID))
	cmdBuildLin := exec.Command("bash", "./build.sh")
	cmdBuildLin.Dir = tempDir
	if out, err := cmdBuildLin.CombinedOutput(); err != nil {
		updateStatus("Linux build failed: %v\n%s", err, string(out))
	} else {
		srcBin := filepath.Join(tempDir, "build", "gorgeous-installer")
		if err := copyFile(srcBin, outBin); err != nil {
			updateStatus("Failed to copy Linux binary: %v", err)
		} else {
			os.Chmod(outBin, 0755)
			updateStatus("Linux build successful!")
		}
	}

	updateStatus("Copying packs and SHA files to output directory...")
	cmdCpPacks := exec.Command("cp", "-R", filepath.Join(packsDir, ".")+"/", outDir+"/")
	if err := cmdCpPacks.Run(); err != nil {
		updateStatus("Failed to copy packs to output directory: %v", err)
	} else {
		updateStatus("Packs and SHA files copied successfully.")
	}

	updateStatus("Offline publisher build completed! Files written to %s", outDir)

	fyne.Do(func() {
		if progress != nil {
			progress.Hide()
		}
		dialog.ShowInformation("Offline Publish Complete", fmt.Sprintf("Standalone installer successfully built!\n\nExecutables generated in:\n%s", outDir), win)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDirFiltered(src, dst string, excludePaths []string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		normRel := strings.ReplaceAll(rel, "\\", "/")

		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "Binaries" || base == "Intermediate" || base == "Saved" || base == "DerivedDataCache" || base == ".vs" {
				return filepath.SkipDir
			}
		}

		for _, excl := range excludePaths {
			normExcl := strings.ReplaceAll(excl, "\\", "/")
			if strings.HasPrefix(normRel, normExcl) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, os.ModePerm)
		}
		return copyFile(path, target)
	})
}

func copyDir(src, dst string) error {
	return copyDirFiltered(src, dst, nil)
}
