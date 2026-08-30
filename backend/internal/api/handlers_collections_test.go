package api

import (
	"net/http"
	"testing"
)

func TestCollectionsAndFilters(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, "PUT", env.server.URL+"/api/users/me/filters", token,
		map[string]any{"filters": map[string]any{"q": "space", "tag": "scifi"}})
	if status != http.StatusOK {
		t.Fatalf("save filters status = %d, body = %v", status, d)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/users/me/filters", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get filters status = %d", status)
	}
	filters, _ := d["filters"].(map[string]any)
	if filters["q"] != "space" || filters["tag"] != "scifi" {
		t.Fatalf("filters = %v", filters)
	}

	status, d = doJSON(t, "POST", env.server.URL+"/api/collections", token,
		map[string]any{"name": "Sci-Fi", "query": map[string]any{"tag": "scifi"}})
	if status != http.StatusCreated {
		t.Fatalf("create collection status = %d, body = %v", status, d)
	}
	colID, _ := d["id"].(string)

	status, d = doJSON(t, "GET", env.server.URL+"/api/collections", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list collections status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("collection count = %d, want 1", len(items))
	}

	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/collections/"+colID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete collection status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/collections", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list collections after delete status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("collection count after delete = %d, want 0", len(items))
	}
}
