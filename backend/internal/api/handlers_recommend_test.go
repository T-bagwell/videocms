package api

import (
	"context"
	"net/http"
	"testing"
)

func TestSimilarVideosAndTagCloud(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	v1 := env.insertVideo(t, libID, "Alpha", ".mkv")
	v2 := env.insertVideo(t, libID, "Beta", ".mkv")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET genres='{Sci-Fi}' WHERE id=$1`, v1); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET genres='{Sci-Fi}' WHERE id=$1`, v2); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{v1.String(), v2.String()} {
		status, _ := doJSON(t, "POST", env.server.URL+"/api/videos/"+id+"/tags", token,
			map[string]any{"name": "space"})
		if status != http.StatusCreated {
			t.Fatalf("add tag status = %d", status)
		}
	}

	status, d := doJSON(t, "GET", env.server.URL+"/api/videos/"+v1.String()+"/similar", token, nil)
	if status != http.StatusOK {
		t.Fatalf("similar status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) == 0 {
		t.Fatal("similar returned no videos")
	}
	found := false
	for _, it := range items {
		if it.(map[string]any)["id"] == v2.String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("similar did not include Beta: %v", items)
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/tags", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tag cloud status = %d", status)
	}
	items, _ = d["items"].([]any)
	found = false
	for _, it := range items {
		m := it.(map[string]any)
		if m["name"] == "space" && m["count"].(float64) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("tag cloud missing space(2): %v", items)
	}
}
