package api

import (
	"net/http"
	"testing"
)

func TestFuzzySearch(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	env.insertVideo(t, libID, "Interstellar", ".mkv")
	env.insertVideo(t, libID, "Dune", ".mkv")

	// A typo does not match substring mode…
	status, d := doJSON(t, "GET", env.server.URL+"/api/videos?q=interselar", token, nil)
	if status != http.StatusOK {
		t.Fatalf("search status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("substring mode matched typo: %v", items)
	}

	// …but fuzzy mode returns it via trigram similarity.
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos?q=interselar&sort=fuzzy", token, nil)
	if status != http.StatusOK {
		t.Fatalf("fuzzy search status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) == 0 {
		t.Fatal("fuzzy search returned no videos for typo")
	}
	found := false
	for _, it := range items {
		if it.(map[string]any)["title"] == "Interstellar" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fuzzy search did not return Interstellar: %v", items)
	}

	// Plain substring search still works.
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos?q=stellar", token, nil)
	if status != http.StatusOK {
		t.Fatalf("substring search status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("substring search items = %d, want 1", len(items))
	}
}
