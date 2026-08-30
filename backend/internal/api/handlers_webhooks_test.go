package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookDeliveryAndOpenAPI(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	var got map[string]any
	var signature string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		signature = r.Header.Get("X-Videocms-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer hook.Close()

	status, _ := doJSON(t, "POST", env.server.URL+"/api/admin/webhooks", token,
		map[string]any{"url": hook.URL, "secret": "s3cret", "events": []string{"upload.completed"}, "active": true})
	if status != http.StatusCreated {
		t.Fatalf("create webhook status = %d", status)
	}

	target := t.TempDir()
	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "w.mp4", "target_path": target, "size": 3})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d", status)
	}
	id := d["id"].(string)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/uploads/%s/chunk/0", env.server.URL, id), bytes.NewReader([]byte("abc")))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ := http.DefaultClient.Do(req)
	_ = res.Body.Close()
	status, _ = doJSON(t, "POST", env.server.URL+"/api/uploads/"+id+"/complete", token, nil)
	if status != http.StatusOK {
		t.Fatalf("complete status = %d", status)
	}

	if got == nil || got["event"] != "upload.completed" {
		t.Fatalf("webhook event = %v", got)
	}
	if !strings.HasPrefix(signature, "sha256=") {
		t.Fatalf("signature = %q, want sha256=…", signature)
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/openapi.json", token, nil)
	if status != http.StatusOK || d["openapi"] != "3.0.3" {
		t.Fatalf("openapi status = %d, body = %v", status, d)
	}
}
