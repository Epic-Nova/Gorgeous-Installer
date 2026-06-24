package ui

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/go-piv/piv-go/piv"
)

func (g *GUIApp) performPIVSign(win fyne.Window, challenge string, appendStatus func(string, ...any), onCancel func()) (string, error) {
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
		if onCancel != nil {
			onCancel()
		}
		return "", fmt.Errorf("cancelled (no PIN provided)")
	}

	appendStatus("Signing challenge with YubiKey (PIV)...")

	cards, err := piv.Cards()
	if err != nil || len(cards) == 0 {
		return "", fmt.Errorf("no smart cards found: %v", err)
	}

	var yk *piv.YubiKey
	var ykErr error
	for i := 0; i < 3; i++ {
		yk, ykErr = piv.Open(cards[0])
		if ykErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	err = ykErr

	if err != nil {
		killChan := make(chan bool)
		fyne.Do(func() {
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
			yk, ykErr = piv.Open(cards[0])
			err = ykErr
		}

		if err != nil {
			return "", fmt.Errorf("failed to open YubiKey: %v", err)
		}
	}
	defer yk.Close()

	cert, err := yk.Certificate(piv.SlotSignature)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate from Slot 9C: %v", err)
	}

	priv, err := yk.PrivateKey(piv.SlotSignature, cert.PublicKey, piv.KeyAuth{PIN: pin})
	if err != nil {
		return "", fmt.Errorf("authentication failed (wrong PIN?): %v", err)
	}

	hash := sha256.Sum256([]byte(challenge))
	signer, ok := priv.(crypto.Signer)
	if !ok {
		return "", fmt.Errorf("key is not a valid signer")
	}

	sig, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("sign failed: %v", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nil
}
