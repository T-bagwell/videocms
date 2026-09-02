package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestTraktStatusAndNotConfigured(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/admin/trakt/status", token, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if d["configured"] != false {
		t.Fatalf("configured = %v, want false", d["configured"])
	}

	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/trakt/sync", token, nil)
	if status != http.StatusBadRequest || !strings.Contains(d["error"].(string), "TRAKT_CLIENT_ID") {
		t.Fatalf("sync without config status = %d body = %v", status, d)
	}
}
