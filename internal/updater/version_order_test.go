package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorgeous-installer/internal/api"
)

func TestUpdateCheckUsesVersionComponents(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		update          bool
	}{
		{"1.9.99", "1.10.0", true}, {"1.10.0", "1.9.99", false},
		{"2.7.9-Dev.4", "2.7.9", true}, {"2.7.9", "2.7.9-Dev.4", false},
		{"2.7.9-Dev.9", "2.7.9-Dev.10", true}, {"2.7.9+local", "v2.7.9+published", false},
	} {
		t.Run(tc.current+" to "+tc.latest, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/installer/update-check" {
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(api.InstallerUpdateResponse{UpdateAvailable: true, LatestVersion: tc.latest})
			}))
			defer server.Close()
			oldURL, oldTransport := api.BaseURL, http.DefaultTransport
			api.BaseURL, http.DefaultTransport = server.URL, server.Client().Transport
			defer func() { api.BaseURL, http.DefaultTransport = oldURL, oldTransport }()
			t.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
			t.Setenv("GORGEOUS_OFFLINE", "")
			_, update := CheckForUpdates(tc.current, true)
			if update != tc.update {
				t.Fatalf("update=%v, want %v", update, tc.update)
			}
		})
	}
}
