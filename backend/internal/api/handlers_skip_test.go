package api

import (
	"net/http"
	"testing"
)

func TestSkipIntervalCRUD(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Skip Me", ".mkv")

	// Empty by default.
	status, d := doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/skip-intervals", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get status = %d", status)
	}
	if d["intro"] != nil || d["credits"] != nil {
		t.Fatalf("expected empty intervals, got %v", d)
	}

	// Create an intro interval.
	status, d = doJSON(t, "PUT", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval", token,
		map[string]any{"kind": "intro", "start_sec": 12.5, "end_sec": 95})
	if status != http.StatusOK {
		t.Fatalf("put status = %d, body = %v", status, d)
	}
	if d["kind"] != "intro" || d["start_sec"] != 12.5 || d["end_sec"] != 95.0 {
		t.Fatalf("unexpected response: %v", d)
	}

	// Upsert replaces the interval.
	status, d = doJSON(t, "PUT", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval", token,
		map[string]any{"kind": "intro", "start_sec": 10, "end_sec": 90})
	if status != http.StatusOK {
		t.Fatalf("upsert status = %d", status)
	}

	// Both kinds visible after adding credits.
	status, d = doJSON(t, "PUT", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval", token,
		map[string]any{"kind": "credits", "start_sec": 3600, "end_sec": 3750})
	if status != http.StatusOK {
		t.Fatalf("put credits status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/skip-intervals", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get status = %d", status)
	}
	intro, _ := d["intro"].(map[string]any)
	credits, _ := d["credits"].(map[string]any)
	if intro == nil || credits == nil {
		t.Fatalf("expected both intervals, got %v", d)
	}
	if intro["start_sec"] != 10.0 || intro["end_sec"] != 90.0 {
		t.Fatalf("intro interval mismatch: %v", intro)
	}
	if credits["start_sec"] != 3600.0 || credits["end_sec"] != 3750.0 {
		t.Fatalf("credits interval mismatch: %v", credits)
	}

	// Delete credits only.
	status, d = doJSON(t, "DELETE", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval?kind=credits", token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/skip-intervals", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get after delete status = %d", status)
	}
	if d["credits"] != nil || d["intro"] == nil {
		t.Fatalf("expected credits cleared, intro kept: %v", d)
	}

	// Invalid requests.
	for name, body := range map[string]map[string]any{
		"bad kind":   {"kind": "trailer", "start_sec": 1, "end_sec": 2},
		"negative":   {"kind": "intro", "start_sec": -1, "end_sec": 2},
		"zero width": {"kind": "intro", "start_sec": 5, "end_sec": 5},
		"reversed":   {"kind": "intro", "start_sec": 9, "end_sec": 2},
	} {
		status, _ = doJSON(t, "PUT", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval", token, body)
		if status != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, status)
		}
	}
	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/videos/"+videoID.String()+"/skip-interval?kind=poster", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("bad delete kind: status = %d, want 400", status)
	}
	status, _ = doJSON(t, "GET", env.server.URL+"/api/videos/not-a-uuid/skip-intervals", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("bad video id: status = %d, want 400", status)
	}
}
