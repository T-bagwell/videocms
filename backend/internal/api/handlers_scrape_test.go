package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"videocms/backend/internal/media"
)

func TestCustomScraperProvider(t *testing.T) {
	env := newIntegrationEnv(t)
	hit := 0
	var backdropURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"title": "Custom Title", "year": 2021, "synopsis": "Custom synopsis",
			"genres":        []string{"Sci-Fi"},
			"trailer_url":   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			"trailer_title": "Official Trailer",
			"backdrop_url":  backdropURL,
		})
	}))
	defer srv.Close()
	backdropURL = srv.URL + "/backdrop.jpg"
	env.app.scraper = media.NewScraper(env.pool, env.app.cfg.DataDir, "", srv.URL)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Original", ".mkv")
	token := loginAdmin(t, env)
	base := env.server.URL + "/api/videos/" + videoID.String() + "/scrape"

	status, d := doJSON(t, "POST", base+"?provider=custom", token, nil)
	if status != http.StatusOK {
		t.Fatalf("custom scrape status = %d, body = %v", status, d)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String(), token, nil)
	if status != http.StatusOK || d["title"] != "Custom Title" {
		t.Fatalf("video after scrape status = %d, body = %v", status, d)
	}
	if d["trailer_url"] != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Fatalf("trailer_url after scrape = %v, want youtube watch URL", d["trailer_url"])
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/backdrop", token, nil)
	if status != http.StatusOK {
		t.Fatalf("scraped backdrop status = %d body = %v", status, d)
	}

	// Without force, an already-enriched video is rejected with 409.
	status, d = doJSON(t, "POST", base+"?provider=custom", token, nil)
	if status != http.StatusConflict {
		t.Fatalf("repeat scrape status = %d, want 409 (body %v)", status, d)
	}
	// Force overwrites.
	status, _ = doJSON(t, "POST", base+"?provider=custom&force=1", token, nil)
	if status != http.StatusOK {
		t.Fatalf("forced scrape status = %d, want 200", status)
	}
}

func TestCustomScraperNotConfigured(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.scraper = media.NewScraper(env.pool, env.app.cfg.DataDir, "")
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "x", ".mkv")
	token := loginAdmin(t, env)
	status, d := doJSON(t, "POST",
		env.server.URL+"/api/videos/"+videoID.String()+"/scrape?provider=custom", token, nil)
	if status != http.StatusBadGateway {
		t.Fatalf("unconfigured custom status = %d, body = %v", status, d)
	}
}
