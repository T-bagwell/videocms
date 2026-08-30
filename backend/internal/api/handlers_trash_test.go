package api

import (
	"net/http"
	"testing"
)

func TestBatchAndTrash(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	v1 := env.insertVideo(t, libID, "A", ".mkv")
	v2 := env.insertVideo(t, libID, "B", ".mkv")

	status, d := doJSON(t, "POST", env.server.URL+"/api/admin/videos/batch", token,
		map[string]any{"ids": []string{v1.String(), v2.String()}, "action": "tag", "tag": "watchlist"})
	if status != http.StatusOK {
		t.Fatalf("batch tag status = %d, body = %v", status, d)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+v1.String()+"/tags", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tags status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("tags after batch = %d, want 1", len(items))
	}

	status, _ = doJSON(t, "POST", env.server.URL+"/api/admin/videos/batch", token,
		map[string]any{"ids": []string{v1.String()}, "action": "delete"})
	if status != http.StatusOK {
		t.Fatalf("batch delete status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/admin/trash", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list trash status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("trash count = %d, want 1", len(items))
	}
	trashID := items[0].(map[string]any)["id"].(string)

	status, d = doJSON(t, "POST", env.server.URL+"/api/admin/trash/"+trashID+"/restore", token, nil)
	if status != http.StatusOK {
		t.Fatalf("restore status = %d, body = %v", status, d)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+v1.String(), token, nil)
	if status != http.StatusOK || d["available"] != true {
		t.Fatalf("video after restore status = %d, body = %v", status, d)
	}

	status, _ = doJSON(t, "POST", env.server.URL+"/api/admin/videos/batch", token,
		map[string]any{"ids": []string{v2.String()}, "action": "clear_tags"})
	if status != http.StatusOK {
		t.Fatalf("batch clear tags status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+v2.String()+"/tags", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tags after clear status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("tags after clear = %d, want 0", len(items))
	}
}
