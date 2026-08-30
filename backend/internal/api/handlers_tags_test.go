package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeFakeAITagger(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-ai")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nprintf 'action\\nsci-fi\\naction\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVideoTagsAndAnalyze(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.cfg.AITagBin = writeFakeAITagger(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Tagged", ".mkv")
	token := loginAdmin(t, env)
	base := env.server.URL + "/api/videos/" + videoID.String()

	status, d := doJSON(t, "POST", base+"/analyze", token, nil)
	if status != http.StatusOK {
		t.Fatalf("analyze status = %d, body = %v", status, d)
	}
	tags, _ := d["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("analyze tags = %v, want 2 (duplicates filtered)", tags)
	}

	status, d = doJSON(t, "GET", base+"/tags", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list tags status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("tag count = %d, want 2", len(items))
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/videos?tag=action", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tag filter status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) == 0 {
		t.Fatal("tag filter returned no videos")
	}

	status, d = doJSON(t, "POST", base+"/tags", token, map[string]any{"name": "Mine"})
	if status != http.StatusCreated {
		t.Fatalf("add tag status = %d, body = %v", status, d)
	}
	tagID, _ := d["id"].(string)
	status, _ = doJSON(t, "DELETE", base+"/tags/"+tagID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("remove tag status = %d", status)
	}
}

func TestAnalyzeRequiresConfig(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "x", ".mkv")
	token := loginAdmin(t, env)
	status, _ := doJSON(t, "POST", env.server.URL+"/api/videos/"+videoID.String()+"/analyze", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("unconfigured analyze status = %d, want 400", status)
	}
}
