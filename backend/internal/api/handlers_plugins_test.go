package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginsDirectoryAndDelivery(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/plugins/directory", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) == 0 {
		t.Fatalf("directory status = %d body = %v", status, d)
	}

	received := make(chan map[string]any, 1)
	header := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header <- r.Header.Get("X-Videocms-Plugin")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- body
	}))
	defer srv.Close()

	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/plugins", token,
		map[string]any{
			"name":        "test-hook",
			"description": "test",
			"install_url": srv.URL,
			"kind":        "webhook",
			"events":      []string{"scan"},
		})
	if status != http.StatusCreated {
		t.Fatalf("install status = %d body = %v", status, d)
	}

	env.app.notifyEvent("scan", "Scan done", "Library scan finished", map[string]any{"library": "lib"})

	if got := <-header; got != "test-hook" {
		t.Fatalf("plugin header = %q", got)
	}
	body := <-received
	if body["event"] != "scan" || body["library"] != "lib" {
		t.Fatalf("plugin payload = %v", body)
	}

	// Disabling stops delivery.
	pluginID := d["id"].(string)
	status, _ = doJSON(t, http.MethodPatch,
		env.server.URL+"/api/admin/plugins/"+pluginID, token,
		map[string]any{"enabled": false})
	if status != http.StatusOK {
		t.Fatalf("disable status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/plugins", token, nil)
	item := d["items"].([]any)[0].(map[string]any)
	if item["enabled"] != false {
		t.Fatalf("plugin after disable = %v", item)
	}
}
