package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestPublisherUploadRetainsConsentAndTLS(t *testing.T) {
	var uploads atomic.Int32
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/systems/sample/publish" {
			fmt.Fprintf(w, `{"Success":true,"UploadUrl":%q}`, server.URL+"/upload")
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/upload" {
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != "sample payload" {
				t.Errorf("unexpected upload %q: %v", body, err)
			}
			uploads.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	oldURL, oldTransport := BaseURL, http.DefaultTransport
	BaseURL, http.DefaultTransport = server.URL, server.Client().Transport
	defer func() { BaseURL, http.DefaultTransport = oldURL, oldTransport }()
	t.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
	t.Setenv("GORGEOUS_OFFLINE", "")
	payload := filepath.Join(t.TempDir(), "sample payload.zip")
	if err := os.WriteFile(payload, []byte("sample payload"), 0600); err != nil {
		t.Fatal(err)
	}
	publish := func() error { return PublishSystem("sample", "1.0", "", "", "", payload, "", nil, nil, nil) }
	if err := publish(); err != nil {
		t.Fatal(err)
	}
	if uploads.Load() != 1 {
		t.Fatal("payload was not uploaded")
	}
	t.Setenv("GORGEOUS_OFFLINE", "1")
	if err := publish(); err == nil {
		t.Fatal("offline publish was accepted")
	}
	if uploads.Load() != 1 {
		t.Fatal("offline publish reached upload endpoint")
	}
}
