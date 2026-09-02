package api

import (
	"net/http"
	"testing"
)

func TestDownloadEnhancementsStored(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/downloads", token,
		map[string]any{
			"url":          "https://example.com/@channel",
			"target_path":  t.TempDir(),
			"format":       "best",
			"kind":         "channel",
			"proxy":        "http://proxy:3128",
			"cookies_path": "/tmp/cookies.txt",
			"username":     "user",
			"password":     "pass",
		})
	if status != http.StatusCreated {
		t.Fatalf("create status = %d body = %v", status, d)
	}
	if d["kind"] != "channel" || d["proxy"] != "http://proxy:3128" ||
		d["username"] != "user" || d["cookies_path"] != "/tmp/cookies.txt" {
		t.Fatalf("stored job = %v", d)
	}
}
