package ui

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/go-piv/piv-go/piv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gorgeous-installer/internal/api"
	"gorgeous-installer/internal/settings"
	"gorgeous-installer/internal/config"
)

// SystemManifest represents the JSON manifest required for GT ecosystem packs.
type SystemManifest struct {
	ID           string   `json:"SystemId"`
	Version      string   `json:"Version"`
	Name         string   `json:"Name"`
	Description  string   `json:"Description"`
	IsCoreSystem bool     `json:"bIsCoreSystem"`
	PayloadPaths []string `json:"PayloadPaths"`
}

func (g *GUIApp) buildPublisherPanel(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	_, err := settings.LoadSettings()
	if err != nil {
		appendStatus("Warning: failed to load settings for publisher: %v", err)
	}

	var loadedManifest *SystemManifest

	var versions []versionEntry

	packManifestIDLbl := newGTValueLabel("-")
	packManifestNameLbl := newGTValueLabel("-")
	pluginManifestIDLbl := newGTValueLabel("-")
	pluginManifestNameLbl := newGTValueLabel("-")

	changelogEntry := widget.NewEntry()
	changelogEntry.SetPlaceHolder("e.g. https://github.com/user/repo/releases/tag/v1.0.0")

	var publishBtn, offlinePublishBtn *accentButton
	installerType := "Source"
	
	// History List (Mock for now, will connect to API)
	historyList := container.NewVBox(
		newGTLabel("Fetching release history..."),
	)
	
	refreshHistory := func(id string) {
		historyList.Objects = []fyne.CanvasObject{
			newGTLabel("History for " + id + ":"),
			newGTCard("REPLACE_ME_WITH_DATA", "Published 2 days ago", newGTLabel("- Added new UI features")),
			newGTCard("REPLACE_ME_WITH_DATA", "Published 2 weeks ago", newGTLabel("- Initial release")),
		}
		historyList.Refresh()
	}

	packEntriesBox := container.NewVBox()
	pluginEntriesBox := container.NewVBox()

	var updateList func()
	updateList = func() {
		packEntriesBox.Objects = nil
		pluginEntriesBox.Objects = nil
		for i, v := range versions {
			idx := i
			lbl := widget.NewLabel(fmt.Sprintf("UE %s -> %s", v.ueVer, filepath.Base(v.sourcePath)))
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				versions = append(versions[:idx], versions[idx+1:]...)
				updateList()
				if len(versions) == 0 {
					loadedManifest = nil
					setCanvasText(packManifestIDLbl, "-")
					setCanvasText(packManifestNameLbl, "-")
					setCanvasText(pluginManifestIDLbl, "-")
					setCanvasText(pluginManifestNameLbl, "-")
					if publishBtn != nil {
						publishBtn.SetEnabled(false)
					}
					if offlinePublishBtn != nil {
						offlinePublishBtn.SetEnabled(false)
					}
				}
			})
			packEntriesBox.Add(container.NewHBox(lbl, layout.NewSpacer(), delBtn))

			lblPlugin := widget.NewLabel(fmt.Sprintf("UE %s -> %s", v.ueVer, filepath.Base(v.sourcePath)))
			delBtnPlugin := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				versions = append(versions[:idx], versions[idx+1:]...)
				updateList()
				if len(versions) == 0 {
					loadedManifest = nil
					setCanvasText(packManifestIDLbl, "-")
					setCanvasText(packManifestNameLbl, "-")
					setCanvasText(pluginManifestIDLbl, "-")
					setCanvasText(pluginManifestNameLbl, "-")
					if publishBtn != nil {
						publishBtn.SetEnabled(false)
					}
					if offlinePublishBtn != nil {
						offlinePublishBtn.SetEnabled(false)
					}
				}
			})
			pluginEntriesBox.Add(container.NewHBox(lblPlugin, layout.NewSpacer(), delBtnPlugin))
		}
		packEntriesBox.Refresh()
		pluginEntriesBox.Refresh()
	}
	
	ueVerEntry := widget.NewEntry()
	ueVerEntry.SetPlaceHolder("e.g. 5.4")

	sysVerEntry := widget.NewEntry()
	sysVerEntry.SetPlaceHolder("Semantic version, e.g. 1.0.0")

	installerSysVerEntry := widget.NewEntry()
	installerSysVerEntry.SetPlaceHolder("Semantic version, e.g. 1.0.0")

	pluginUeVerEntry := widget.NewEntry()
	pluginUeVerEntry.SetPlaceHolder("e.g. 5.4 or Universal")

	pluginSysVerEntry := widget.NewEntry()
	pluginSysVerEntry.SetPlaceHolder("Plugin version, e.g. 1.0.0")

	pluginPathEntry := widget.NewEntry()
	pluginPathEntry.SetPlaceHolder("Path to Plugin Folder...")



	pluginPathBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if lu != nil {
				pluginPathEntry.SetText(lu.Path())
			}
		}, win)
	})

	infoBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		dialog.ShowInformation("Version Info", "Enter a specific Unreal Engine version (e.g. '5.4') or 'Universal' to skip engine version matching for plugins that natively support multiple versions.", win)
	})

	pluginAddBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		if pluginUeVerEntry.Text == "" || pluginPathEntry.Text == "" {
			return
		}

		dialog.ShowInformation("Reminder: Version Compatibility", "Before deploying, ensure you have checked version compatibility across the core general systems!\n\nPrecheck on every Unreal version you want to publish if the blueprint systems that natively ship with plugin updates work. If not, you must deploy a patch for those versions alongside the Universal one.", win)

		pluginID, friendlyName, versionName, err := ParsePluginInfo(pluginPathEntry.Text)
		if err != nil {
			g.showAnimatedDialog("Error", err.Error(), true)
			return
		}

		if loadedManifest == nil {
			loadedManifest = &SystemManifest{
				ID:   pluginID,
				Name: friendlyName,
			}
			setCanvasText(packManifestIDLbl, pluginID)
			setCanvasText(packManifestNameLbl, friendlyName)
			setCanvasText(pluginManifestIDLbl, pluginID)
			setCanvasText(pluginManifestNameLbl, friendlyName)
			if pluginSysVerEntry.Text == "" {
				pluginSysVerEntry.SetText(versionName)
			}
			refreshHistory(pluginID)
			if publishBtn != nil {
				publishBtn.SetEnabled(true)
			}
			if offlinePublishBtn != nil {
				offlinePublishBtn.SetEnabled(true)
			}
		}

		versions = append(versions, versionEntry{
			ueVer:      pluginUeVerEntry.Text,
			sourcePath: pluginPathEntry.Text,
		})
		pluginUeVerEntry.SetText("")
		pluginPathEntry.SetText("")
		if updateList != nil {
			updateList()
		}
	})

	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("Path to System Folder...")

	pathBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
			if lu != nil {
				pathEntry.SetText(lu.Path())
			}
		}, win)
	})

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), func() {
		if ueVerEntry.Text == "" || pathEntry.Text == "" {
			return
		}

		manifestPath := filepath.Join(pathEntry.Text, "SystemManifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			g.showAnimatedDialog("Error", "SystemManifest.json not found in selected directory", true)
			return
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			g.showAnimatedDialog("Error", "failed to read SystemManifest.json", true)
			return
		}
		var localM SystemManifest
		if err := json.Unmarshal(data, &localM); err != nil {
			g.showAnimatedDialog("Error", "failed to parse SystemManifest.json", true)
			return
		}

		if loadedManifest == nil {
			loadedManifest = &localM
			setCanvasText(packManifestIDLbl, localM.ID)
			setCanvasText(packManifestNameLbl, localM.Name)
			setCanvasText(pluginManifestIDLbl, localM.ID)
			setCanvasText(pluginManifestNameLbl, localM.Name)
			if sysVerEntry.Text == "" {
				sysVerEntry.SetText(localM.Version)
			}
			refreshHistory(localM.ID)
			if publishBtn != nil {
				publishBtn.SetEnabled(true)
			}
			if offlinePublishBtn != nil {
				offlinePublishBtn.SetEnabled(true)
			}
		}

		versions = append(versions, versionEntry{
			ueVer:      ueVerEntry.Text,
			sourcePath: pathEntry.Text,
		})
		ueVerEntry.SetText("")
		pathEntry.SetText("")
		updateList()
	})

	inputRow := container.NewVBox(
		container.NewHBox(
			widget.NewLabel("UE Ver:"), container.NewGridWrap(fyne.NewSize(70, 35), ueVerEntry),
			widget.NewLabel("Path:"), container.NewGridWrap(fyne.NewSize(200, 35), pathEntry), pathBrowseBtn,
			layout.NewSpacer(),
			addBtn,
		),
	)

	var publishMode string = "Pack Update"

	installerPathEntry := widget.NewEntry()
	installerPath := "."
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		if _, err := os.Stat("../go.mod"); err == nil {
			installerPath = ".."
		}
	}
	// Resolve to absolute so the entry is unambiguous and build.sh can always be found.
	if abs, err := filepath.Abs(installerPath); err == nil {
		installerPath = abs
	}
	installerPathEntry.SetText(installerPath)

	publishBtn = newAccentButton("Sign & Publish", accentUpdate, func() {
		var sysID, sysName, sysDesc string
		isInstallerBinary := false

		runPublish := func() {
			notes := changelogEntry.Text
			if strings.TrimSpace(notes) == "" {
				g.showAnimatedDialog("Error", "please provide a release notes URL", true)
				return
			}

			publishBtn.SetRunning(true)
			publishBtn.SetEnabled(false)

			// Create and show progress dialog synchronously on the main thread.
			// Do NOT wrap in fyne.Do here — the goroutine below would otherwise
			// race against the deferred Show and see a nil progress pointer,
			// causing the dialog to flash and vanish instantly.
			statusLbl := widget.NewLabel("Starting publish workflow for " + sysName + "...")
			statusLbl.Wrapping = fyne.TextWrapWord
			progBar := widget.NewProgressBar()
			progBar.Max = 4
			progBar.SetValue(0)
			content := container.NewVBox(statusLbl, progBar)
			progress := dialog.NewCustom("Publish Progress", "Hide to Background", content, win)
			progress.Show()

			updateStatus := func(msg string, args ...any) {
				text := fmt.Sprintf(msg, args...)
				appendStatus("%s", text)
				fyne.Do(func() {
					if statusLbl != nil {
						statusLbl.SetText(text)
					}
				})
			}

			go func() {
				zipPath := filepath.Join(os.TempDir(), sysID+"-payload.zip")
				// 1. Fetch Challenge
				updateStatus("1. Fetching cryptographic challenge from API...")
				challenge, err := api.GetPublishChallenge(sysID)
				
				var regData *api.SystemRegistrationData
				if err != nil {
					if err == api.ErrSystemNotFound {
						regChan := make(chan *api.SystemRegistrationData)
						fyne.Do(func() {
							if progress != nil {
								progress.Hide()
							}
							targetPluginEntry := widget.NewEntry()
						targetPluginEntry.SetText(sysID)
						displayNameEntry := widget.NewEntry()
						displayNameEntry.SetText(sysName)
						descEntry := widget.NewMultiLineEntry()
						descEntry.SetText(sysDesc)
						isCoreCheck := widget.NewCheck("Is Core System?", nil)
						
						formItems := []*widget.FormItem{
							widget.NewFormItem("Target Plugin", targetPluginEntry),
							widget.NewFormItem("Display Name", displayNameEntry),
							widget.NewFormItem("Description", descEntry),
						}
						if publishMode == "Pack Update" {
							formItems = append(formItems, widget.NewFormItem("", isCoreCheck))
						}
						
						dialog.ShowForm("System Not Found - Register as new Pack?", "Register", "Cancel", formItems, func(b bool) {
							if b {
								regChan <- &api.SystemRegistrationData{
									TargetPluginName: targetPluginEntry.Text,
									DisplayName:      displayNameEntry.Text,
									Description:      descEntry.Text,
									IsCoreSystem:     publishMode == "Pack Update" && isCoreCheck.Checked,
								}
							} else {
								regChan <- nil
							}
						}, win)
						})
						
						regData = <-regChan
						if regData == nil {
							fyne.Do(func() {
								appendStatus("Publish cancelled (registration aborted).")
								publishBtn.SetRunning(false)
								publishBtn.SetEnabled(true)
							})
							return
						}
						
						fyne.Do(func() {
							if progress != nil {
								progress.Show()
							}
						})
						// The challenge fetched from the first call is already valid and will be signed.
					} else {
						fyne.Do(func() {
							if progress != nil {
								progress.Hide()
							}
							appendStatus("Failed to fetch challenge: %v", err)
							publishBtn.SetRunning(false)
							publishBtn.SetEnabled(true)
						})
						return
					}
				}
				fyne.Do(func() { if progBar != nil { progBar.SetValue(1) } })

				// 2. Build hybrid pack and zip
				updateStatus("2. Building hybrid packs and zipping...")

				tempDir, err := os.MkdirTemp("", "gorgeous-online-*")
				if err != nil {
					fyne.Do(func() {
						if progress != nil {
							progress.Hide()
						}
						appendStatus("Failed to create temp dir: %v", err)
						publishBtn.SetRunning(false)
						publishBtn.SetEnabled(true)
					})
					return
				}
				defer os.RemoveAll(tempDir)

				packsDir := filepath.Join(tempDir, "packs")
				os.MkdirAll(packsDir, 0755)

				var payloadChecksum string

				if publishMode == "Installer Update" {
					updateStatus("2. Zipping installer payload...")

					// Read srcDir synchronously — installerPathEntry already holds
					// the absolute path after the rebuild step patched it.
					srcDir := installerPathEntry.Text
					if srcDir == "" {
						srcDir = "."
					}
					
					var cmdZip *exec.Cmd
					if !isInstallerBinary {
						cmdZip = exec.Command("zip", "-r", zipPath, ".", "-x", "build/*", "build", "*.exe", "*.syso", ".git/*", ".git", "*.log", "*.gti")
						cmdZip.Dir = srcDir
					} else {
						var buildDir string
						if filepath.Base(srcDir) == "build" {
							buildDir = srcDir
						} else {
							buildDir = filepath.Join(srcDir, "build")
							if _, err := os.Stat(buildDir); os.IsNotExist(err) {
								if _, errBin := os.Stat(filepath.Join(srcDir, "gorgeous-installer")); errBin == nil {
									buildDir = srcDir
								} else if _, errExe := os.Stat(filepath.Join(srcDir, "gorgeous-installer.exe")); errExe == nil {
									buildDir = srcDir
								} else {
									fyne.Do(func() {
										if progress != nil {
											progress.Hide()
										}
										appendStatus("Zip failed: chdir build: no such file or directory. Please build first.")
										publishBtn.SetRunning(false)
										publishBtn.SetEnabled(true)
									})
									return
								}
							}
						}
						cmdZip = exec.Command("zip", "-r", zipPath, ".")
						cmdZip.Dir = buildDir
					}
					if err := cmdZip.Run(); err != nil {
						fyne.Do(func() {
							if progress != nil {
								progress.Hide()
							}
							appendStatus("Zip failed: %v", err)
							publishBtn.SetRunning(false)
							publishBtn.SetEnabled(true)
						})
						return
					}
					
					// Compute hash of the zip payload for the checksum
					if zipBytes, err := os.ReadFile(zipPath); err == nil {
						h := sha256.Sum256(zipBytes)
						payloadChecksum = hex.EncodeToString(h[:])
					}
				} else {
				// PLUGIN OR PACK UPDATE MODE
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
					packName := fmt.Sprintf("%s-%s", loadedManifest.ID, v.ueVer)
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

					var exclusions []string

					if publishMode == "Plugin Update" {
						// Exclude all systems where !IsCoreSystem
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

						// For Plugin update, copy the whole vPluginRoot
						copyDirFiltered(vPluginRoot, packPath, exclusions)

					} else {
						// Pack Update Mode
						var pathsToCopy []string
						if len(loadedManifest.PayloadPaths) > 0 {
							pathsToCopy = loadedManifest.PayloadPaths
						} else {
							pathsToCopy = []string{"."}
						}

						for _, relPath := range pathsToCopy {
							src := filepath.Join(vPluginRoot, relPath)
							dst := filepath.Join(packPath, relPath)
							
							if info, err := os.Stat(src); err == nil {
								if info.IsDir() {
									os.MkdirAll(dst, 0755)
									copyDirFiltered(src, dst, exclusions)
								} else {
									os.MkdirAll(filepath.Dir(dst), 0755)
									copyFile(src, dst)
								}
							}
						}
					}

					availVersions = append(availVersions, config.PackVersion{
						Version: v.ueVer,
						Path:    fmt.Sprintf("packs/%s", packName),
						SHAFile: fmt.Sprintf("packs/%s.sha256", packName),
					})
				}

			cfg := config.Config{
				PackName:          loadedManifest.ID,
				PackType:          "hybrid",
				PluginName:        actualPluginName,
				AvailableVersions: availVersions,
			}
			cfgData, _ := json.MarshalIndent(cfg, "", "  ")
			os.WriteFile(filepath.Join(tempDir, "config.json"), cfgData, 0644)

			cmdZip := exec.Command("zip", "-r", zipPath, ".")
			cmdZip.Dir = tempDir
			if err := cmdZip.Run(); err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("Zip failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			fyne.Do(func() { if progBar != nil { progBar.SetValue(2) } })
		}
			
		// Prompt for PIN in Fyne GUI
			pinChan := make(chan string)
			fyne.Do(func() {
				entry := widget.NewPasswordEntry()
				dialog.ShowCustomConfirm("YubiKey PIN Required", "Sign", "Cancel", entry, func(b bool) {
					if b {
						pinChan <- entry.Text
					} else {
						pinChan <- ""
					}
				}, win)
			})
			
			pin := <-pinChan
			if pin == "" {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("Publish cancelled (no PIN provided).")
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}

			// 3. Sign using Native PIV Applet
			updateStatus("3. Signing challenge with YubiKey (PIV)...")
			
			cards, err := piv.Cards()
			if err != nil || len(cards) == 0 {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Error: No smart cards found (%v)", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			var yk *piv.YubiKey
			for i := 0; i < 3; i++ {
				yk, err = piv.Open(cards[0])
				if err == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}
			
			if err != nil {
				killChan := make(chan bool)
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					dialog.ShowConfirm("PIV Error", "Failed to open YubiKey (outstanding connections). Force kill conflicting background processes?", func(b bool) {
						killChan <- b
					}, win)
				})
				
				if <-killChan {
					if runtime.GOOS == "windows" {
						exec.Command("taskkill", "/IM", "scardsvr.exe", "/F").Run()
						exec.Command("taskkill", "/IM", "gpg-agent.exe", "/F").Run()
						exec.Command("net", "start", "scardsvr").Run()
					} else {
						exec.Command("killall", "gpg-agent").Run()
					}
					time.Sleep(2 * time.Second)
					yk, err = piv.Open(cards[0])
				}
				
				if err != nil {
					fyne.Do(func() {
						appendStatus("PIV Error: Failed to open YubiKey (%v)", err)
						publishBtn.SetRunning(false)
						publishBtn.SetEnabled(true)
					})
					return
				}
				
				fyne.Do(func() {
					if progress != nil {
						progress.Show()
					}
				})
			}
			defer yk.Close()
			
			// Load the certificate for Slot 9C
			cert, err := yk.Certificate(piv.SlotSignature)
			if err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Error: Failed to read certificate from Slot 9C (%v)", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}

			// Access the private key securely
			priv, err := yk.PrivateKey(piv.SlotSignature, cert.PublicKey, piv.KeyAuth{PIN: pin})
			if err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Error: Authentication failed (wrong PIN?) - %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			// Hash the challenge and sign it
			hash := sha256.Sum256([]byte(challenge))
			signer, ok := priv.(crypto.Signer)
			if !ok {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Error: Key is not a valid signer")
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			sig, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
			
			if err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Sign failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			signature := base64.StdEncoding.EncodeToString(sig)
			updateStatus("PIV Signature acquired successfully.")
			fyne.Do(func() { if progBar != nil { progBar.SetValue(3) } })
			
			sysVer := sysVerEntry.Text
			if publishMode == "Installer Update" {
				sysVer = installerSysVerEntry.Text
			} else if publishMode == "Plugin Update" {
				sysVer = pluginSysVerEntry.Text
			}
			if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
				sysVer = "v" + sysVer
			}

			var minCoreVer string
			if publishMode == "Plugin Update" && len(versions) > 0 {
				minCoreVer = FindMinimumCoreVersion(versions[0].sourcePath)
			}

			// 4. Upload
			updateStatus("4. Uploading payload and changelog to API...")
			err = api.PublishSystem(sysID, sysVer, notes, signature, payloadChecksum, zipPath, minCoreVer, regData, nil, nil)
			if err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("API upload failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			fyne.Do(func() { if progBar != nil { progBar.SetValue(4) } })

			fyne.Do(func() {
				if progress != nil {
					progress.Hide()
				}
				publishBtn.SetRunning(false)
				publishBtn.SetEnabled(true)
				changelogEntry.SetText("")
				appendStatus("Successfully published %s", sysName)
				g.showAnimatedDialog("Published", "Release published successfully.", false)
			})
		}()
	}
		
		if publishMode == "Installer Update" {
			if installerType == "Source" {
				sysID = "GorgeousInstaller-Source"
				sysName = "Gorgeous Installer (Source)"
				sysDesc = "Core Installer Source"
				isInstallerBinary = false
			} else {
				sysID = "GorgeousInstaller-Bin"
				sysName = "Gorgeous Installer (Binary)"
				sysDesc = "Core Installer Binary"
				isInstallerBinary = true
			}
			sysVerForBuild := installerSysVerEntry.Text
			g.promptVersionAndRebuild(win, sysVerForBuild, installerPathEntry.Text, isInstallerBinary, func() {
				runPublish()
			}, appendStatus)
			return
		}

		if loadedManifest == nil {
			return
		}
		sysID = loadedManifest.ID
		sysName = loadedManifest.Name
		sysDesc = loadedManifest.Description
		runPublish()
	})
	publishBtn.SetEnabled(false)

	offlinePublishBtn = newAccentButton("Offline Publish", accentUpdate, func() {
		sysVer := sysVerEntry.Text
		var manifestToPass *SystemManifest = loadedManifest
		
		if publishMode == "Installer Update" {
			sysVer = installerSysVerEntry.Text
			if installerType == "Source" {
				manifestToPass = &SystemManifest{
					ID:   "GorgeousInstaller-Source",
					Name: "Gorgeous Installer (Source)",
				}
			} else {
				manifestToPass = &SystemManifest{
					ID:   "GorgeousInstaller-Bin",
					Name: "Gorgeous Installer (Binary)",
				}
			}
			if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
				sysVer = "v" + sysVer
			}
			manifestForCapture := manifestToPass
			sysVerForCapture := sysVer
			g.promptVersionAndRebuild(win, installerSysVerEntry.Text, installerPathEntry.Text, installerType == "Binary", func() {
				g.showOfflinePublisherDialog(win, publishMode, manifestForCapture, versions, installerPathEntry.Text, sysVerForCapture, appendStatus)
			}, appendStatus)
			return
		}

		if publishMode == "Plugin Update" {
			sysVer = pluginSysVerEntry.Text
		}
		
		if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
			sysVer = "v" + sysVer
		}
		g.showOfflinePublisherDialog(win, publishMode, manifestToPass, versions, installerPathEntry.Text, sysVer, appendStatus)
	})
	offlinePublishBtn.SetEnabled(true)

	installerType = "Source"
	installerTypeSelector := widget.NewRadioGroup([]string{"Source", "Binary"}, func(s string) {
		installerType = s
	})
	installerTypeSelector.Horizontal = true
	installerTypeSelector.SetSelected("Source")

	installerSection := newGTCard("Installer Source", "Select the root directory of the Gorgeous Installer codebase",
		container.NewVBox(
			container.NewHBox(widget.NewLabel("Payload Type:"), installerTypeSelector),
			container.NewHBox(widget.NewLabel("Source Dir:"), container.NewGridWrap(fyne.NewSize(200, 35), installerPathEntry), widget.NewButton("Browse", func() {
				dialog.ShowFolderOpen(func(lu fyne.ListableURI, err error) {
					if lu != nil {
						installerPathEntry.SetText(lu.Path())
					}
				}, win)
			})),
			container.NewHBox(widget.NewLabel("System Version:"), newGTLabel("v"), container.NewGridWrap(fyne.NewSize(120, 35), installerSysVerEntry)),
		),
	)
	installerSection.Hide()

	infoSection := newGTCard("Source Definition", "Map Engine versions to their System folders",
		container.NewVBox(
			container.NewHBox(newGTLabel("System ID:"), packManifestIDLbl, layout.NewSpacer(), newGTLabel("Name:"), packManifestNameLbl),
			container.NewHBox(newGTLabel("System Version:"), newGTLabel("v"), container.NewGridWrap(fyne.NewSize(120, 35), sysVerEntry)),
			widget.NewSeparator(),
			packEntriesBox,
			inputRow,
		),
	)
	
	manageSection := g.buildManageContentSection(win, appendStatus)
	manageSection.Hide()

	var changelogSection, pluginInfoSection fyne.CanvasObject

	clearState := func() {
		versions = nil
		loadedManifest = nil
		setCanvasText(packManifestIDLbl, "-")
		setCanvasText(packManifestNameLbl, "-")
		setCanvasText(pluginManifestIDLbl, "-")
		setCanvasText(pluginManifestNameLbl, "-")
		changelogEntry.SetText("")
		ueVerEntry.SetText("")
		sysVerEntry.SetText("")
		installerSysVerEntry.SetText("")
		pathEntry.SetText("")
		pluginUeVerEntry.SetText("")
		pluginSysVerEntry.SetText("")
		pluginPathEntry.SetText("")
		if updateList != nil {
			updateList()
		}
	}

	modeSelector := widget.NewRadioGroup([]string{"Pack Update", "Installer Update", "Plugin Update", "Manage Content"}, func(s string) {
		if publishMode != s {
			clearState()
		}
		publishMode = s
		if s == "Manage Content" {
			infoSection.Hide()
			pluginInfoSection.Hide()
			installerSection.Hide()
			changelogSection.Hide()
			publishBtn.Hide()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Hide()
			}
			manageSection.Show()
		} else if s == "Installer Update" {
			infoSection.Hide()
			pluginInfoSection.Hide()
			installerSection.Show()
			changelogSection.Show()
			publishBtn.Show()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Show()
			}
			manageSection.Hide()
			publishBtn.SetEnabled(true)
		} else if s == "Plugin Update" {
			infoSection.Hide()
			installerSection.Hide()
			pluginInfoSection.Show()
			changelogSection.Show()
			publishBtn.Show()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Show()
			}
			manageSection.Hide()
			if loadedManifest == nil {
				publishBtn.SetEnabled(false)
			} else {
				publishBtn.SetEnabled(true)
			}
		} else {
			infoSection.Show()
			pluginInfoSection.Hide()
			installerSection.Hide()
			changelogSection.Show()
			publishBtn.Show()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Show()
			}
			manageSection.Hide()
			if loadedManifest == nil {
				publishBtn.SetEnabled(false)
			} else {
				publishBtn.SetEnabled(true)
			}
		}
	})
	modeSelector.Horizontal = true
	
	modeSection := newGTCard("Publish Mode", "Select the type of release", container.NewVBox(modeSelector))

	changelogSection = newGTCard("Release Notes URL", "Link to release notes or changelog",
		container.NewVBox(changelogEntry),
	)

	pluginInputRow := container.NewVBox(
		container.NewHBox(
			widget.NewLabel("UE Ver:"), container.NewGridWrap(fyne.NewSize(70, 35), pluginUeVerEntry), infoBtn,
			widget.NewLabel("Path:"), container.NewGridWrap(fyne.NewSize(200, 35), pluginPathEntry), pluginPathBrowseBtn,
			layout.NewSpacer(),
			pluginAddBtn,
		),
	)

	pluginInfoSection = newGTCard("Plugin Definition", "Map Engine versions to Plugin folders",
		container.NewVBox(
			container.NewHBox(newGTLabel("System ID:"), pluginManifestIDLbl, layout.NewSpacer(), newGTLabel("Name:"), pluginManifestNameLbl),
			container.NewHBox(newGTLabel("Plugin Version:"), newGTLabel("v"), container.NewGridWrap(fyne.NewSize(120, 35), pluginSysVerEntry)),
			widget.NewSeparator(),
			pluginEntriesBox,
			pluginInputRow,
		),
	)
	pluginInfoSection.Hide()

	modeSelector.SetSelected("Pack Update")

	leftPanel := container.NewVScroll(container.NewVBox(
		modeSection,
		manageSection,
		installerSection,
		infoSection,
		pluginInfoSection,
		changelogSection,
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(150, 50), offlinePublishBtn), container.NewGridWrap(fyne.NewSize(200, 50), publishBtn)),
	))

	rightPanel := newGTCard("Release History", "Previous versions on the API",
		container.NewScroll(historyList),
	)

	split := container.NewHSplit(container.NewPadded(leftPanel), container.NewPadded(rightPanel))
	split.Offset = 0.65

	return container.NewPadded(split)
}

// promptVersionAndRebuild asks the user whether to update version strings in
// build.sh and winres.json and rebuild the installer before publishing.
// crossWindows controls whether --cross-windows is passed to build.sh.
// onDone is called when either the rebuild finishes or the user declines.
func (g *GUIApp) promptVersionAndRebuild(win fyne.Window, rawVer string, srcDir string, crossWindows bool, onDone func(), appendStatus func(string, ...any)) {
	// Normalise: strip leading v for build.sh (build.sh doesn't want the v)
	cleanVer := strings.TrimPrefix(rawVer, "v")
	if cleanVer == "" {
		// No version specified – go straight to publishing.
		onDone()
		return
	}

	// Show a SINGLE dialog with two inline buttons.
	// Previously two dialogs were shown back-to-back with an immediate dismissModal()
	// in between. dismissModal() schedules a 300 ms clear of g.modalLayer — that
	// delayed wipe destroyed the second dialog, causing the "vanishes instantly" bug.
	yesBtn := newAccentButton("Update & Rebuild", accentUpdate, func() {
		g.dismissModal()
		g.rebuildInstallerWithVersion(win, cleanVer, srcDir, crossWindows, onDone, appendStatus)
	})
	skipBtn := widget.NewButton("Skip Rebuild", func() {
		g.dismissModal()
		onDone()
	})
	skipBtn.Importance = widget.LowImportance

	body := container.NewVBox(
		widget.NewLabel("Version "+rawVer+" detected. Patch build.sh + winres.json\nand rebuild the installer before publishing?"),
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), skipBtn, yesBtn),
	)
	g.showAnimatedCustomDialog("Rebuild Installer?", body, 520, 185, "Cancel", func() {
		g.dismissModal()
	})
}

// rebuildInstallerWithVersion patches version strings and runs ./build.sh --clean.
// When crossWindows is true, --cross-windows is also passed so the build produces
// both gorgeous-installer (Linux) and gorgeous-installer.exe (Windows) in build/.
func (g *GUIApp) rebuildInstallerWithVersion(win fyne.Window, cleanVer string, srcDir string, crossWindows bool, onDone func(), appendStatus func(string, ...any)) {
	statusLbl := widget.NewLabel("Patching version strings...")
	statusLbl.Wrapping = fyne.TextWrapWord
	progBar := widget.NewProgressBarInfinite()
	content := container.NewVBox(statusLbl, progBar)
	progress := dialog.NewCustom("Building Installer", "Hide", content, win)
	progress.Show()

	updateLbl := func(msg string) {
		appendStatus("%s", msg)
		fyne.Do(func() { statusLbl.SetText(msg) })
	}

	go func() {
		// Resolve srcDir to an absolute path immediately.
		// installerPathEntry may contain a relative path ("." or "..") which
		// is interpreted relative to the binary's working directory at launch
		// time — not necessarily where build.sh lives. Absolute resolution
		// ensures all subsequent filepath.Join and cmd.Dir calls are correct.
		absSrcDir, err := filepath.Abs(srcDir)
		if err != nil {
			updateLbl("Error resolving source directory: " + err.Error())
			fyne.Do(func() { progress.Hide() })
			return
		}
		srcDir = absSrcDir
		updateLbl("Source dir: " + srcDir)

		// --- 1. Patch build.sh ---
		buildShPath := filepath.Join(srcDir, "build.sh")
		if data, err := os.ReadFile(buildShPath); err == nil {
			re := regexp.MustCompile(`(?m)^VERSION="[^"]*"`)
			patched := re.ReplaceAllString(string(data), `VERSION="`+cleanVer+`"`)
			if err := os.WriteFile(buildShPath, []byte(patched), 0755); err != nil {
				updateLbl("Warning: could not patch build.sh: " + err.Error())
			} else {
				updateLbl("Patched build.sh version to " + cleanVer)
			}
		} else {
			updateLbl("Warning: could not read build.sh: " + err.Error())
		}

		// --- 2. Patch winres.json ---
		// winres.json uses a 4-part version string like "1.0.0.0"
		winresPath := filepath.Join(srcDir, "winres.json")
		if data, err := os.ReadFile(winresPath); err == nil {
			// Build a 4-part version from cleanVer (e.g. "1.2.3" -> "1.2.3.0")
			parts := strings.Split(cleanVer, ".")
			for len(parts) < 4 {
				parts = append(parts, "0")
			}
			fourPartVer := strings.Join(parts[:4], ".")
			re := regexp.MustCompile(`"(file_version|product_version|FileVersion|ProductVersion)":\s*"[^"]*"`)
			patched := re.ReplaceAllStringFunc(string(data), func(m string) string {
				keyEnd := strings.Index(m, ":")
				key := m[:keyEnd+1]
				return key + ` "` + fourPartVer + `"`
			})
			if err := os.WriteFile(winresPath, []byte(patched), 0644); err != nil {
				updateLbl("Warning: could not patch winres.json: " + err.Error())
			} else {
				updateLbl("Patched winres.json version to " + fourPartVer)
			}
		} else {
			updateLbl("Warning: could not read winres.json (skipping): " + err.Error())
		}

		// --- 3. Run build ---
		buildArgs := []string{"--clean"}
		if crossWindows {
			buildArgs = append(buildArgs, "--cross-windows")
			updateLbl("Running ./build.sh --clean --cross-windows (Linux + Windows)...")
		} else {
			updateLbl("Running ./build.sh --clean ...")
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// On Windows use build.ps1 (cross-compile not applicable from Windows)
			cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(srcDir, "build.ps1"), "-Clean")
		} else {
			args := append([]string{filepath.Join(srcDir, "build.sh")}, buildArgs...)
			cmd = exec.Command("bash", args...)
		}
		cmd.Dir = srcDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			updateLbl("Build failed: " + err.Error() + "\n" + string(out))
			fyne.Do(func() { progress.Hide() })
			return
		}
		updateLbl("Build succeeded.")
		fyne.Do(func() {
			progress.Hide()
			onDone() // run on main thread so runPublish() can safely create Fyne widgets
		})
	}()
}
