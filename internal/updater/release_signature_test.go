package updater

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"gorgeous-installer/internal/api"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func fixtureSigningKey() ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	return ed25519.NewKeyFromSeed(seed) // Public test vector only; never a production trust key.
}

func TestPerformUpdateRejectsUnsignedReleaseBeforeDownload(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, _ := writer.Create("unexpected.txt")
	entry.Write([]byte("unsigned"))
	writer.Close()
	digest := sha256.Sum256(archive.Bytes())
	var downloads atomic.Int32
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/installer/update-check" {
			json.NewEncoder(w).Encode(map[string]any{"UpdateAvailable": true, "LatestVersion": "1.2.3", "DownloadUrl": server.URL + "/payload.zip", "DownloadToken": server.URL + "/payload.zip", "ChecksumSha256": hex.EncodeToString(digest[:])})
			return
		}
		downloads.Add(1)
		w.Write(archive.Bytes())
	}))
	defer server.Close()
	t.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
	t.Setenv("GORGEOUS_OFFLINE", "")
	t.Setenv("GORGEOUS_RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(fixtureSigningKey().Public().(ed25519.PublicKey)))
	oldBase := api.BaseURL
	api.BaseURL = server.URL
	defer func() { api.BaseURL = oldBase }()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = oldTransport }()
	target := t.TempDir()
	if err := PerformUpdate(target, false); err == nil {
		t.Error("unsigned release was accepted")
	}
	if downloads.Load() != 0 {
		t.Errorf("downloaded %d unsigned payloads", downloads.Load())
	}
	if _, err := os.Stat(filepath.Join(target, "unexpected.txt")); !os.IsNotExist(err) {
		t.Errorf("unsigned release changed installation: %v", err)
	}
}

func TestReleaseSignatureBindsEveryFieldAndPinnedKey(t *testing.T) {
	key := fixtureSigningKey()
	t.Setenv("GORGEOUS_RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
	token, version, digest := "GorgeousAuditToken", "1.2.3", strings.Repeat("0", 64)
	signature := "w70JlZC/abp8+UXKRkqwE1JB883HyThaSoPk9KYKQr8jvBjMcm6zKjyu+mZPVwfxO/ljUVjny3ApSjhLpTDCCA=="
	if err := VerifyReleaseSignature(token, version, digest, signature); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, token, version, digest, signature string }{
		{"token", token + "x", version, digest, signature}, {"version", token, "1.2.4", digest, signature}, {"checksum", token, version, strings.Repeat("1", 64), signature},
		{"signature", token, version, digest, signature[:len(signature)-1]}, {"empty", token, version, digest, ""}, {"newline", token + "\n", version, digest, signature},
		{"carriage", token, version + "\r", digest, signature}, {"nul", token + "\x00", version, digest, signature},
	} {
		t.Run(c.name, func(t *testing.T) {
			if VerifyReleaseSignature(c.token, c.version, c.digest, c.signature) == nil {
				t.Fatal("accepted changed/invalid signed metadata")
			}
		})
	}
	t.Setenv("GORGEOUS_RELEASE_PUBLIC_KEY", "")
	if VerifyReleaseSignature(token, version, digest, signature) == nil {
		t.Fatal("accepted missing trust key")
	}
	other := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
	t.Setenv("GORGEOUS_RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(other.Public().(ed25519.PublicKey)))
	if VerifyReleaseSignature(token, version, digest, signature) == nil {
		t.Fatal("accepted unpinned key")
	}
}
