package updater

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gorgeous-installer/internal/api"
)

func TestPerformUpdateRequiresVerifiedPayload(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("verified.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("verified payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive.Bytes())
	expected := hex.EncodeToString(digest[:])
	for _, test := range []struct {
		name, checksum      string
		insecure, wantError bool
		wantDownloads       int32
	}{
		{name: "valid", checksum: expected, wantDownloads: 1},
		{name: "mismatch", checksum: strings.Repeat("0", 64), wantError: true, wantDownloads: 1},
		{name: "missing", wantError: true},
		{name: "invalid hex", checksum: strings.Repeat("x", 64), wantError: true},
		{name: "insecure URL", checksum: expected, insecure: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			scratch := t.TempDir()
			t.Setenv("TMP", scratch)
			t.Setenv("TEMP", scratch)
			t.Setenv("TMPDIR", scratch)
			t.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
			t.Setenv("GORGEOUS_RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(fixtureSigningKey().Public().(ed25519.PublicKey)))
			t.Setenv("GORGEOUS_OFFLINE", "")
			var downloads atomic.Int32
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/installer/update-check" {
					downloadURL := server.URL + "/payload.zip"
					if test.insecure {
						downloadURL = "http://insecure.invalid/payload.zip"
					}
					signature := base64.StdEncoding.EncodeToString(ed25519.Sign(fixtureSigningKey(), []byte("gorgeous-release-v1\n"+downloadURL+"\n1.2.3\n"+strings.ToLower(test.checksum)+"\n")))
					if err := json.NewEncoder(w).Encode(api.InstallerUpdateResponse{UpdateAvailable: true, LatestVersion: "1.2.3", DownloadUrl: downloadURL, DownloadToken: downloadURL, ChecksumSha256: test.checksum, SignatureEd25519: signature}); err != nil {
						t.Error(err)
					}
					return
				}
				if r.URL.Path != "/payload.zip" {
					http.NotFound(w, r)
					return
				}
				downloads.Add(1)
				if _, err := w.Write(archive.Bytes()); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			oldBase := api.BaseURL
			api.BaseURL = server.URL
			defer func() { api.BaseURL = oldBase }()
			oldTransport := http.DefaultTransport
			http.DefaultTransport = server.Client().Transport
			defer func() { http.DefaultTransport = oldTransport }()
			destination := t.TempDir()
			err := PerformUpdate(destination, false)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v, wantError=%v", err, test.wantError)
			}
			if downloads.Load() != test.wantDownloads {
				t.Errorf("payload downloads=%d, want %d", downloads.Load(), test.wantDownloads)
			}
			data, readErr := os.ReadFile(filepath.Join(destination, "verified.txt"))
			if test.wantError {
				if !os.IsNotExist(readErr) {
					t.Fatalf("rejected payload modified installation: read=%q error=%v", data, readErr)
				}
			} else if readErr != nil || string(data) != "verified payload" {
				t.Fatalf("verified installation: %q %v", data, readErr)
			}
			leftovers, _ := filepath.Glob(filepath.Join(scratch, "*.zip"))
			if len(leftovers) != 0 {
				t.Errorf("temporary archives leaked: %v", leftovers)
			}
		})
	}
}
