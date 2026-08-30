package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"videocms/backend/internal/media"
)

func TestNotifications(t *testing.T) {
	env := newIntegrationEnv(t)
	var mu sync.Mutex
	received := []map[string]any{}
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var d map[string]any
		_ = json.NewDecoder(r.Body).Decode(&d)
		mu.Lock()
		received = append(received, d)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer hook.Close()
	env.app.notify = media.NewNotifier(hook.URL, "")
	token := loginAdmin(t, env)

	status, _ := doJSON(t, "POST", env.server.URL+"/api/admin/notify/test", token, nil)
	if status != http.StatusOK {
		t.Fatalf("test notify status = %d", status)
	}

	target := t.TempDir()
	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "notify.mp4", "target_path": target, "size": 3})
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
		t.Fatalf("complete upload status = %d", status)
	}

	mu.Lock()
	defer mu.Unlock()
	events := map[string]bool{}
	for _, e := range received {
		ev, _ := e["event"].(string)
		events[ev] = true
	}
	if !events["test"] || !events["upload.completed"] {
		t.Fatalf("received events = %v", events)
	}
}
