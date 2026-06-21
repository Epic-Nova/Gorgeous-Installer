package ui

import (
	"encoding/json"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"crypto"
	"crypto/rand"
	"crypto/sha256"

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

	publishBtn = newAccentButton("Sign & Publish", accentUpdate, func() {
		if loadedManifest == nil {
			return
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
			statusLbl = widget.NewLabel("Starting publish workflow for " + loadedManifest.Name + "...")
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
			zipPath := filepath.Join(os.TempDir(), loadedManifest.ID+"-payload.zip")
			// 1. Fetch Challenge
			updateStatus("1. Fetching cryptographic challenge from API...")
			challenge, err := api.GetPublishChallenge(loadedManifest.ID)
			if err != nil {
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
							copyDir(src, dst)
						} else {
							os.MkdirAll(filepath.Dir(dst), 0755)
							copyFile(src, dst)
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
			
			yk, err := piv.Open(cards[0])
			if err != nil {
				fyne.Do(func() {
					if progress != nil {
						progress.Hide()
					}
					appendStatus("PIV Error: Failed to open YubiKey (%v)", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			// Load the certificate for Slot 9C
			cert, err := yk.Certificate(piv.SlotSignature)
			if err != nil {
				yk.Close()
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
				yk.Close()
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
				yk.Close()
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
			yk.Close() // Release the YubiKey
			
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
			if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
				sysVer = "v" + sysVer
			}

			// 4. Upload
			updateStatus("4. Uploading payload and changelog to API...")
			err = api.PublishSystem(loadedManifest.ID, sysVer, notes, signature, zipPath)
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
				appendStatus("Successfully published %s v%s", loadedManifest.Name, loadedManifest.Version)
				g.showAnimatedDialog("Published", "Release published successfully.", false)
			})
		}()
	})
	publishBtn.SetEnabled(false)

	offlinePublishBtn = newAccentButton("Offline Publish", accentUpdate, func() {
		sysVer := sysVerEntry.Text
		if sysVer != "" && !strings.HasPrefix(sysVer, "v") {
			sysVer = "v" + sysVer
		}
		g.showOfflinePublisherDialog(win, loadedManifest, versions, sysVer, appendStatus)
	})
	offlinePublishBtn.SetEnabled(true)

	infoSection := newGTCard("Source Definition", "Map Engine versions to their System folders",
		container.NewVBox(
			container.NewHBox(newGTLabel("System ID:"), manifestIDLbl, layout.NewSpacer(), newGTLabel("Name:"), manifestNameLbl),
			container.NewHBox(newGTLabel("System Version:"), container.NewGridWrap(fyne.NewSize(120, 35), sysVerEntry)),
			widget.NewSeparator(),
			entriesBox,
			inputRow,
		),
	)

	changelogSection := newGTCard("Release Notes", "Format as a git commit message",
		container.NewGridWrap(fyne.NewSize(400, 150), changelogEntry),
	)

	leftPanel := container.NewVScroll(container.NewVBox(
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
