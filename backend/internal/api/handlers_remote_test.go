package api

import (
	"net/http"
	"testing"

	"videocms/backend/internal/config"
)

func TestRemoteAccessTurn(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/remote-access", token, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if d["turn"] != nil {
		t.Fatalf("turn should be null when unconfigured: %v", d)
	}

	env2 := newIntegrationEnv(t, func(c *config.Config) {
		c.TurnServer = "turn:turn.example.com:3478?transport=udp"
		c.TurnUsername = "user"
		c.TurnPassword = "secret"
	})
	token2 := loginAdmin(t, env2)
	status, d = doJSON(t, http.MethodGet, env2.server.URL+"/api/remote-access", token2, nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	turn := d["turn"].(map[string]any)
	if turn["url"] != "turn:turn.example.com:3478?transport=udp" ||
		turn["username"] != "user" || turn["credential"] != "secret" {
		t.Fatalf("turn = %v", turn)
	}
}
