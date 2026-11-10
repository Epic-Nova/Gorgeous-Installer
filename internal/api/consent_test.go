package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestConsentAndTLS(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	old := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = old }()
	t.Setenv("GORGEOUS_ALLOW_NETWORK", "")
	t.Setenv("GORGEOUS_OFFLINE", "")
	req, _ := http.NewRequest("GET", server.URL, nil)
	if _, err := (consentTransport{}).RoundTrip(req); err == nil {
		t.Fatal("default-off request allowed")
	}
	if hits.Load() != 0 {
		t.Fatal("denied request reached server")
	}
	t.Setenv("GORGEOUS_ALLOW_NETWORK", "1")
	resp, err := (consentTransport{}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if hits.Load() != 1 {
		t.Fatal("consented HTTPS request failed")
	}
	plain, _ := http.NewRequest("GET", "http://api.gorgeous.simsalabim.studio/", nil)
	if _, err := (consentTransport{}).RoundTrip(plain); err == nil {
		t.Fatal("HTTP downgrade allowed")
	}
	t.Setenv("GORGEOUS_OFFLINE", "1")
	if _, err := (consentTransport{}).RoundTrip(req); err == nil {
		t.Fatal("offline did not override consent")
	}
	if hits.Load() != 1 {
		t.Fatal("offline request reached server")
	}
}
