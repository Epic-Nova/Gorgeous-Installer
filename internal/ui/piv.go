//go:build cgo

package ui

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
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
	if err != nil {
		return "", fmt.Errorf("failed to enumerate smart cards: %w", err)
	}
	if len(cards) == 0 {
		return "", fmt.Errorf("no smart cards found")
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
		return "", fmt.Errorf("failed to open YubiKey: %w; check the smart-card service and close conflicting card sessions before retrying", err)
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
