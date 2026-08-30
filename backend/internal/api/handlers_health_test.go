package api

import (
	"context"
	"net/http"
	"testing"
)

func TestLibraryHealthCheckAndKeepBest(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	v1 := env.insertVideo(t, libID, "dup-a", ".mkv")
	v2 := env.insertVideo(t, libID, "dup-b", ".mkv")
	// both fake files have the same size (1234), so they are duplicate candidates

	status, d := doJSON(t, "POST", env.server.URL+"/api/libraries/"+libID.String()+"/health", token, nil)
	if status != http.StatusOK {
		t.Fatalf("health status = %d, body = %v", status, d)
	}
	dups, _ := d["duplicates"].([]any)
	if len(dups) != 1 {
		t.Fatalf("duplicate groups = %d, want 1 (body %v)", len(dups), d)
	}
	group := dups[0].(map[string]any)
	if group["count"].(float64) != 2 {
		t.Fatalf("duplicate group count = %v, want 2", group["count"])
	}

	status, d = doJSON(t, "POST",
		env.server.URL+"/api/libraries/"+libID.String()+"/health/keep-best", token, nil)
	if status != http.StatusOK {
		t.Fatalf("keep-best status = %d, body = %v", status, d)
	}
	moved, _ := d["moved"].([]any)
	if len(moved) != 1 {
		t.Fatalf("moved = %d, want 1 (body %v)", len(moved), d)
	}
	var remaining bool
	_ = env.pool.QueryRow(context.Background(),
		`SELECT available FROM videos WHERE id=$1`, v2).Scan(&remaining)
	// v2 was moved unless v1 was considered better; either way exactly one stays.
	var total int
	_ = env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM videos WHERE id IN ($1,$2) AND available`, v1, v2).Scan(&total)
	if total != 1 {
		t.Fatalf("available videos after keep-best = %d, want 1", total)
	}
}

func TestLibraryHealthMissingFile(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "gone", ".mkv")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET file_path='/nonexistent/videocms-missing.mp4' WHERE id=$1`, videoID); err != nil {
		t.Fatal(err)
	}
	status, d := doJSON(t, "POST", env.server.URL+"/api/libraries/"+libID.String()+"/health", token, nil)
	if status != http.StatusOK {
		t.Fatalf("health status = %d", status)
	}
	missing, _ := d["missing"].([]any)
	if len(missing) != 1 {
		t.Fatalf("missing = %d, want 1 (body %v)", len(missing), d)
	}
}
