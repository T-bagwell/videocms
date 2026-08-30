package api

import (
	"net/http"
	"testing"
)

func TestJobsAndSystem(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()
	libID := env.insertLibrary(t, false)
	env.insertVideo(t, libID, "JobVid", ".mkv")

	status, _ := doJSON(t, "POST", env.server.URL+"/api/downloads", token,
		map[string]any{"url": "https://example.com/v", "target_path": target})
	if status != http.StatusCreated {
		t.Fatalf("create download status = %d", status)
	}
	status, _ = doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "j.mp4", "target_path": target, "size": 1})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d", status)
	}

	status, d := doJSON(t, "GET", env.server.URL+"/api/admin/jobs", token, nil)
	if status != http.StatusOK {
		t.Fatalf("jobs status = %d", status)
	}
	items, _ := d["items"].([]any)
	kinds := map[string]bool{}
	for _, it := range items {
		kinds[it.(map[string]any)["kind"].(string)] = true
	}
	for _, want := range []string{"scan", "upload", "download"} {
		if !kinds[want] {
			t.Fatalf("jobs missing kind %q: %v", want, kinds)
		}
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/admin/system", token, nil)
	if status != http.StatusOK {
		t.Fatalf("system status = %d", status)
	}
	if d["disk_total"].(float64) <= 0 {
		t.Fatalf("disk_total = %v, want > 0", d["disk_total"])
	}
}
