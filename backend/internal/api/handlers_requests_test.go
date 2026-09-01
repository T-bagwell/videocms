package api

import (
	"context"
	"net/http"
	"testing"
)

func TestRequestWorkflow(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/requests", token,
		map[string]any{"title": "Dune: Part Two", "year": 2024, "media_type": "movie", "notes": "Please"})
	if status != http.StatusCreated {
		t.Fatalf("create request status = %d body = %v", status, d)
	}
	id := d["id"].(string)

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/requests", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("my requests status = %d body = %v", status, d)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/requests/all", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("all requests status = %d body = %v", status, d)
	}

	// Approve without a download.
	status, _ = doJSON(t, http.MethodPost, env.server.URL+"/api/requests/"+id+"/decide", token,
		map[string]any{"status": "approved"})
	if status != http.StatusOK {
		t.Fatalf("decide status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/requests", token, nil)
	item := d["items"].([]any)[0].(map[string]any)
	if item["status"] != "approved" {
		t.Fatalf("status after approve = %v", item["status"])
	}

	// Reject a second request.
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/requests", token,
		map[string]any{"title": "Old Movie"})
	if status != http.StatusCreated {
		t.Fatal("create second request failed")
	}
	id2 := d["id"].(string)
	status, _ = doJSON(t, http.MethodPost, env.server.URL+"/api/requests/"+id2+"/decide", token,
		map[string]any{"status": "rejected"})
	if status != http.StatusOK {
		t.Fatalf("reject status = %d", status)
	}

	// Approve with a download URL feeds the download queue.
	libID := env.insertLibrary(t, false)
	target := t.TempDir()
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/requests", token,
		map[string]any{"title": "New Release"})
	if status != http.StatusCreated {
		t.Fatal("create third request failed")
	}
	id3 := d["id"].(string)
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/requests/"+id3+"/decide", token,
		map[string]any{"status": "approved", "download_url": "https://example.com/movie.mp4", "target_path": target})
	if status != http.StatusOK {
		t.Fatalf("approve with download status = %d body = %v", status, d)
	}
	if d["status"] != "downloading" {
		t.Fatalf("status with download = %v, want downloading", d["status"])
	}
	var n int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM downloads WHERE url=$1`, "https://example.com/movie.mp4").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("downloads queued = %d, want 1", n)
	}
	_ = libID
}
