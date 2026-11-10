package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// PinnedReleasePublicKey may be supplied by a developer-owned build via -ldflags -X.
// The API response is never a source of trust keys.
var PinnedReleasePublicKey string

func VerifyReleaseSignature(token, version, checksum, signature string) error {
	if token == "" || version == "" || strings.ContainsAny(token+version, "\r\n\x00") {
		return fmt.Errorf("invalid signed release identity")
	}
	digest, err := hex.DecodeString(checksum)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("invalid signed release SHA-256")
	}
	encodedKey := os.Getenv("GORGEOUS_RELEASE_PUBLIC_KEY")
	if encodedKey == "" {
		encodedKey = PinnedReleasePublicKey
	}
	key, err := base64.StdEncoding.Strict().DecodeString(encodedKey)
	if err != nil || len(key) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(key) != encodedKey {
		return fmt.Errorf("missing or invalid pinned release public key")
	}
	signed, err := base64.StdEncoding.Strict().DecodeString(signature)
	if err != nil || len(signed) != ed25519.SignatureSize || base64.StdEncoding.EncodeToString(signed) != signature {
		return fmt.Errorf("missing or invalid release signature")
	}
	payload := "gorgeous-release-v1\n" + token + "\n" + version + "\n" + strings.ToLower(checksum) + "\n"
	if !ed25519.Verify(ed25519.PublicKey(key), []byte(payload), signed) {
		return fmt.Errorf("release signature verification failed")
	}
	return nil
}
