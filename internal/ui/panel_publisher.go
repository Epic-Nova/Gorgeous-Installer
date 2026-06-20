package ui

import (
	"encoding/json"
	"encoding/base64"
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

	sourcePathLbl := newGTValueLabel("No directory selected")
	manifestIDLbl := newGTValueLabel("-")
	manifestVerLbl := newGTValueLabel("-")
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

	selectSourceBtn := widget.NewButtonWithIcon("Select Pack/Plugin Folder", theme.FolderOpenIcon(), func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			p := normalizeURIPath(lu)
			if p != "" {
				setCanvasText(sourcePathLbl, p)

				// Look for SystemManifest.json
				manifestPath := filepath.Join(p, "SystemManifest.json")
				data, err := os.ReadFile(manifestPath)
				if err == nil {
					var m SystemManifest
					if json.Unmarshal(data, &m) == nil {
						loadedManifest = &m
						setCanvasText(manifestIDLbl, m.ID)
						setCanvasText(manifestVerLbl, m.Version)
						setCanvasText(manifestNameLbl, m.Name)
						publishBtn.SetEnabled(true)
						if offlinePublishBtn != nil {
							offlinePublishBtn.SetEnabled(true)
						}
						refreshHistory(m.ID)
						appendStatus("Loaded manifest for %s v%s", m.Name, m.Version)
						return
					}
				}
				
				// Fallback to uplugin?
				publishBtn.SetEnabled(false)
				setCanvasText(manifestIDLbl, "Error")
				setCanvasText(manifestVerLbl, "Error")
				setCanvasText(manifestNameLbl, "Error")
				appendStatus("SystemManifest.json not found or invalid in %s", p)
				g.showAnimatedDialog("Error", "SystemManifest.json not found in selected directory", true)
			}
		}, win)
		d.Show()
	})

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
		appendStatus("Starting publish workflow for %s v%s...", loadedManifest.Name, loadedManifest.Version)

		go func() {
			p := sourcePathLbl.Text
			zipPath := filepath.Join(os.TempDir(), loadedManifest.ID+"-payload.zip")

			// 1. Fetch Challenge
			appendStatus("1. Fetching cryptographic challenge from API...")
			challenge, err := api.GetPublishChallenge(loadedManifest.ID)
			if err != nil {
				fyne.Do(func() {
					appendStatus("Failed to fetch challenge: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}

			// 2. Zip
			appendStatus("2. Zipping source files...")
			
			pluginRoot := p
			for pluginRoot != "" && pluginRoot != string(filepath.Separator) && pluginRoot != "." {
				matches, _ := filepath.Glob(filepath.Join(pluginRoot, "*.uplugin"))
				if len(matches) > 0 {
					break
				}
				parent := filepath.Dir(pluginRoot)
				if parent == pluginRoot {
					pluginRoot = p
					break
				}
				pluginRoot = parent
			}

			args := []string{"-r", zipPath}
			if len(loadedManifest.PayloadPaths) > 0 {
				args = append(args, loadedManifest.PayloadPaths...)
			} else {
				rel, err := filepath.Rel(pluginRoot, p)
				if err == nil && rel != "." && rel != "" {
					args = append(args, rel)
				} else {
					args = append(args, ".")
				}
			}

			cmdZip := exec.Command("zip", args...)
			cmdZip.Dir = pluginRoot
			if err := cmdZip.Run(); err != nil {
				fyne.Do(func() {
					appendStatus("Zip failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
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
					appendStatus("Publish cancelled (no PIN provided).")
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}

			// 3. Sign using Native PIV Applet
			appendStatus("3. Signing challenge with YubiKey (PIV)...")
			
			cards, err := piv.Cards()
			if err != nil || len(cards) == 0 {
				fyne.Do(func() {
					appendStatus("PIV Error: No smart cards found (%v)", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			yk, err := piv.Open(cards[0])
			if err != nil {
				fyne.Do(func() {
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
					appendStatus("PIV Sign failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}
			
			signature := base64.StdEncoding.EncodeToString(sig)
			appendStatus("PIV Signature acquired successfully.")
			
			// 4. Upload
			appendStatus("4. Uploading payload and changelog to API...")
			err = api.PublishSystem(loadedManifest.ID, loadedManifest.Version, notes, signature, zipPath)
			if err != nil {
				fyne.Do(func() {
					appendStatus("API upload failed: %v", err)
					publishBtn.SetRunning(false)
					publishBtn.SetEnabled(true)
				})
				return
			}

			fyne.Do(func() {
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
		g.showOfflinePublisherDialog(win, loadedManifest, sourcePathLbl.Text, appendStatus)
	})
	offlinePublishBtn.SetEnabled(false)

	infoSection := newGTCard("Source Definition", "Select the directory containing SystemManifest.json",
		container.NewVBox(
			container.NewHBox(selectSourceBtn, layout.NewSpacer()),
			container.NewBorder(nil, nil, newGTLabel("Path:"), nil, container.NewHScroll(sourcePathLbl)),
			container.NewHBox(newGTLabel("ID:"), manifestIDLbl, layout.NewSpacer(), newGTLabel("Version:"), manifestVerLbl),
			container.NewHBox(newGTLabel("Name:"), manifestNameLbl),
		),
	)

	changelogSection := newGTCard("Release Notes", "Format as a git commit message",
		container.NewGridWrap(fyne.NewSize(400, 150), changelogEntry),
	)

	leftPanel := container.NewVBox(
		infoSection,
		changelogSection,
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(150, 50), offlinePublishBtn), container.NewGridWrap(fyne.NewSize(200, 50), publishBtn)),
	)

	rightPanel := newGTCard("Release History", "Previous versions on the API",
		container.NewScroll(historyList),
	)

	split := container.NewHSplit(container.NewPadded(leftPanel), container.NewPadded(rightPanel))
	split.Offset = 0.65

	return container.NewPadded(split)
}
