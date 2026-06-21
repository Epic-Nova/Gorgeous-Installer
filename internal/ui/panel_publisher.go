package ui

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"strings"

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

	manifestIDLbl := newGTValueLabel("-")
	manifestNameLbl := newGTValueLabel("-")

	changelogEntry := widget.NewMultiLineEntry()
	changelogEntry.SetPlaceHolder("Enter release notes here (git commit style)...")

	var publishBtn, offlinePublishBtn *accentButton
	
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

	entriesBox := container.NewVBox()

	var updateList func()
	updateList = func() {
		entriesBox.Objects = nil
		for i, v := range versions {
			idx := i
			lbl := widget.NewLabel(fmt.Sprintf("UE %s -> %s", v.ueVer, filepath.Base(v.sourcePath)))
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				versions = append(versions[:idx], versions[idx+1:]...)
				updateList()
				if len(versions) == 0 {
					loadedManifest = nil
					setCanvasText(manifestIDLbl, "-")
					setCanvasText(manifestNameLbl, "-")
					if publishBtn != nil {
						publishBtn.SetEnabled(false)
					}
					if offlinePublishBtn != nil {
						offlinePublishBtn.SetEnabled(false)
					}
				}
			})
			entriesBox.Add(container.NewHBox(lbl, layout.NewSpacer(), delBtn))
		}
		entriesBox.Refresh()
	}

	ueVerEntry := widget.NewEntry()
	ueVerEntry.SetPlaceHolder("e.g. 5.4")

	sysVerEntry := widget.NewEntry()
	sysVerEntry.SetPlaceHolder("System version, e.g. 1.0.0")

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
			setCanvasText(manifestIDLbl, localM.ID)
			setCanvasText(manifestNameLbl, localM.Name)
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
	installerPathEntry.SetText(".") // default to current dir
	
	installerSysVerEntry := widget.NewEntry()
	installerSysVerEntry.SetPlaceHolder("System version, e.g. 1.0.0")

	installerSubSelector := widget.NewRadioGroup([]string{"Source", "Binary"}, func(s string) {
	})
	installerSubSelector.Horizontal = true
	installerSubSelector.SetSelected("Source")

	publishBtn = newAccentButton("Sign & Publish", accentUpdate, func() {
		var sysID, sysName, sysDesc string
		if publishMode == "Installer Update" {
			if installerSubSelector.Selected == "Source" {
				sysID = "GorgeousInstaller-Source"
				sysName = "Gorgeous Installer Source"
				sysDesc = "Core Installer Source Code"
			} else {
				sysID = "GorgeousInstaller-Bin"
				sysName = "Gorgeous Installer Binaries"
				sysDesc = "Compiled Installer Binaries"
			}
		} else {
			if loadedManifest == nil {
				return
			}
			sysID = loadedManifest.ID
			sysName = loadedManifest.Name
			sysDesc = loadedManifest.Description
		}
		
		notes := changelogEntry.Text
		if strings.TrimSpace(notes) == "" {
			g.showAnimatedDialog("Error", "please provide release notes", true)
			return
		}

		publishBtn.SetRunning(true)
		publishBtn.SetEnabled(false)

		var progress *dialog.CustomDialog
		var statusLbl *widget.Label
		var progBar *widget.ProgressBar

		fyne.Do(func() {
			statusLbl = widget.NewLabel("Starting publish workflow for " + sysName + "...")
			statusLbl.Wrapping = fyne.TextWrapWord
			progBar = widget.NewProgressBar()
			progBar.Min = 0
			progBar.Max = 4
			progBar.SetValue(0)
			content := container.NewVBox(statusLbl, progBar)
			progress = dialog.NewCustom("Publish Progress", "Hide to Background", content, win)
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
							widget.NewFormItem("", isCoreCheck),
						}
						
						dialog.ShowForm("System Not Found - Register as new Pack?", "Register", "Cancel", formItems, func(b bool) {
							if b {
								regChan <- &api.SystemRegistrationData{
									TargetPluginName: targetPluginEntry.Text,
									DisplayName:      displayNameEntry.Text,
									Description:      descEntry.Text,
									IsCoreSystem:     isCoreCheck.Checked,
								}
							} else {
								regChan <- nil
							}
						}, win)
					})
					
					regData = <-regChan
					if regData == nil {
						fyne.Do(func() {
							appendStatus("Publish cancelled by user.")
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
			var actualPluginName string
			var availVersions []config.PackVersion
			
			

			if publishMode == "Installer Source Update" || publishMode == "Installer Binary Update" {
				updateStatus("2. Zipping installer payload...")
				
				var srcDir string
				fyne.Do(func() {
					srcDir = installerPathEntry.Text
				})
				// fallback if empty
				if srcDir == "" {
					srcDir = "."
				}
				
				var cmdZip *exec.Cmd
				if publishMode == "Installer Update" {
					if installerSubSelector.Selected == "Source" {
						cmdZip = exec.Command("zip", "-r", zipPath, ".", "-x", "build/*", "*.exe", "*.syso", ".git/*", "*.log")
						cmdZip.Dir = srcDir
					} else {
						buildDir := filepath.Join(srcDir, "build")
						cmdZip = exec.Command("zip", "-r", zipPath, ".")
						cmdZip.Dir = buildDir
					}
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
				
								actualPluginName = filepath.Base(firstPluginRoot)

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
			
			var sysVer string
			if publishMode == "Installer Update" {
				sysVer = installerSysVerEntry.Text
			} else {
				sysVer = sysVerEntry.Text
			}
			if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
				sysVer = "v" + sysVer
			}

			// 4. Upload
			updateStatus("4. Uploading payload and changelog to API...")
			err = api.PublishSystem(sysID, sysVer, notes, signature, payloadChecksum, zipPath, regData)
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
	})
	publishBtn.SetEnabled(false)

	offlinePublishBtn = newAccentButton("Offline Publish", accentUpdate, func() {
		var sysVer string
		if publishMode == "Installer Update" {
			sysVer = installerSysVerEntry.Text
		} else {
			sysVer = sysVerEntry.Text
		}
		if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
			sysVer = "v" + sysVer
		}
		g.showOfflinePublisherDialog(win, loadedManifest, versions, sysVer, appendStatus)
	})
	offlinePublishBtn.SetEnabled(true)

	installerSubRow := container.NewHBox(widget.NewLabel("Update Type:"), installerSubSelector)

	installerSection := newGTCard("Installer Source", "Select the root directory of the Gorgeous Installer codebase",
		container.NewVBox(
			installerSubRow,
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
			container.NewHBox(newGTLabel("System ID:"), manifestIDLbl, layout.NewSpacer(), newGTLabel("Name:"), manifestNameLbl),
			container.NewHBox(newGTLabel("System Version:"), newGTLabel("v"), container.NewGridWrap(fyne.NewSize(120, 35), sysVerEntry)),
			widget.NewSeparator(),
			entriesBox,
			inputRow,
		),
	)
	
	changelogSection := newGTCard("Release Notes", "Format as a git commit message",
		container.NewGridWrap(fyne.NewSize(400, 150), changelogEntry),
	)
	
	manageSection := g.buildManageSection(win, appendStatus)
	manageSection.Hide()

	modeSelector := widget.NewRadioGroup([]string{"Pack Update", "Installer Update", "Plugin Update", "Manage Packs"}, func(s string) {
		publishMode = s
		if s == "Manage Packs" {
			infoSection.Hide()
			installerSection.Hide()
			changelogSection.Hide()
			publishBtn.Hide()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Hide()
			}
			manageSection.Show()
		} else if s == "Installer Update" {
			infoSection.Hide()
			installerSection.Show()
			changelogSection.Show()
			publishBtn.Show()
			if offlinePublishBtn != nil {
				offlinePublishBtn.Show()
			}
			manageSection.Hide()
			publishBtn.SetEnabled(true)
		} else {
			infoSection.Show()
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
	modeSelector.SetSelected("Pack Update")
	
	modeSection := newGTCard("Publish Mode", "Select the type of release", container.NewVBox(modeSelector))

	leftPanel := container.NewVScroll(container.NewVBox(
		modeSection,
		manageSection,
		installerSection,
		infoSection,
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
