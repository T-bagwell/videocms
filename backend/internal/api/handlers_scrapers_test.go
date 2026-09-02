package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalScraperSDK(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"title":    "External Title",
			"year":     req["year"],
			"synopsis": "Fetched by an external scraper",
			"genres":   []string{"External"},
		})
	}))
	defer srv.Close()

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/admin/scrapers", token,
		map[string]any{"name": "ext", "kind": "url", "url": srv.URL + "/%s"})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d body = %v", status, d)
	}
	scraperID := d["id"].(string)

	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Original", ".mkv")
	status, d = doJSON(t, http.MethodPost,
		env.server.URL+"/api/videos/"+videoID.String()+"/scrape?provider=ext", token, nil)
	if status != http.StatusOK {
		t.Fatalf("scrape with external status = %d body = %v", status, d)
	}
	status, d = doJSON(t, http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String(), token, nil)
	if status != http.StatusOK || d["title"] != "External Title" {
		t.Fatalf("video after external scrape = %v", d)
	}

	// Disabling the scraper rejects further scrapes.
	status, _ = doJSON(t, http.MethodPatch,
		env.server.URL+"/api/admin/scrapers/"+scraperID, token,
		map[string]any{"enabled": false})
	if status != http.StatusOK {
		t.Fatalf("disable status = %d", status)
	}
	video2 := env.insertVideo(t, libID, "Another", ".mkv")
	status, d = doJSON(t, http.MethodPost,
		env.server.URL+"/api/videos/"+video2.String()+"/scrape?provider=ext&force=1", token, nil)
	if status != http.StatusBadGateway {
		t.Fatalf("disabled scraper status = %d body = %v", status, d)
	}
	if !containsStr(fmt.Sprint(d["error"]), "not found or disabled") {
		t.Fatalf("error = %v", d["error"])
	}
}
