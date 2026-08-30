package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestLiveStreamAndChat(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.cfg.RTMPIngestURL = "rtmp://localhost:1935/live"
	token := loginAdmin(t, env)

	status, d := doJSON(t, "POST", env.server.URL+"/api/live", token, map[string]any{"title": "My Stream"})
	if status != http.StatusCreated {
		t.Fatalf("create live status = %d, body = %v", status, d)
	}
	id, _ := d["id"].(string)
	ingest, _ := d["ingest_url"].(string)
	if id == "" || ingest == "" {
		t.Fatalf("create live missing id/ingest_url: %v", d)
	}
	if !strings.Contains(ingest, "/live/") {
		t.Fatalf("ingest URL should contain the stream key path: %q", ingest)
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/live", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list live status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("live stream count = %d, want 1", len(items))
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/live/"+id, token, nil)
	if status != http.StatusOK || d["title"] != "My Stream" {
		t.Fatalf("get live status = %d, body = %v", status, d)
	}

	status, d = doJSON(t, "POST", env.server.URL+"/api/live/"+id+"/chat", token,
		map[string]any{"body": "hello live"})
	if status != http.StatusCreated {
		t.Fatalf("send chat status = %d, body = %v", status, d)
	}
	if d["username"] != "admin" {
		t.Fatalf("chat username = %v", d["username"])
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/live/"+id+"/chat", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list chat status = %d", status)
	}
	msgs, _ := d["items"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("chat count = %d, want 1", len(msgs))
	}

	status, _ = doJSON(t, "POST", env.server.URL+"/api/live/"+id+"/chat", token,
		map[string]any{"body": ""})
	if status != http.StatusBadRequest {
		t.Fatalf("empty chat status = %d, want 400", status)
	}
}
