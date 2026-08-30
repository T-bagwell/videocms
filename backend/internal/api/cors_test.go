package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"videocms/backend/internal/config"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORSWildcardByDefault(t *testing.T) {
	app := &App{}
	rec := httptest.NewRecorder()
	app.cors(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want *", got)
	}
}

func TestCORSAllowList(t *testing.T) {
	app := &App{cfg: config.Config{CORSOrigins: []string{"https://app.example.com"}}}
	h := app.cors(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allowed origin = %q", got)
	}
	if got := rec.Header().Values("Vary"); len(got) == 0 || got[0] != "Origin" {
		t.Fatalf("Vary = %v, want Origin", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("denied origin got Allow-Origin = %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodOptions, "/api/videos", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	app.cors(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("preflight missing Allow-Methods")
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got == "" {
		t.Fatal("missing Expose-Headers for Range streaming")
	}
}
