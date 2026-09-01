package api

import (
	"net/http"
	"testing"

	"videocms/backend/internal/config"
)

func TestRegistrationClosed(t *testing.T) {
	env := newIntegrationEnv(t, func(c *config.Config) { c.RegistrationEnabled = false })
	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/auth/register", "",
		map[string]any{"username": "newuser", "password": "secret1"})
	if status != http.StatusForbidden {
		t.Fatalf("closed registration status = %d body = %v", status, d)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/auth/registration", "", nil)
	if status != http.StatusOK || d["enabled"] != false {
		t.Fatalf("policy status = %d body = %v", status, d)
	}
}

func TestInviteOnlyRegistration(t *testing.T) {
	env := newIntegrationEnv(t, func(c *config.Config) { c.RegistrationInviteOnly = true })
	token := loginAdmin(t, env)

	// Without an invite code registration is rejected.
	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/auth/register", "",
		map[string]any{"username": "newuser", "password": "secret1"})
	if status != http.StatusBadRequest {
		t.Fatalf("no-code status = %d body = %v", status, d)
	}

	// Generate an invite and use it.
	status, _ = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/invites", token,
		map[string]any{"count": 2})
	if status != http.StatusOK {
		t.Fatalf("generate invites status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/invites", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 2 {
		t.Fatalf("list invites status = %d body = %v", status, d)
	}
	code := d["items"].([]any)[0].(map[string]any)["code"].(string)
	id := d["items"].([]any)[0].(map[string]any)["id"].(string)

	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/auth/register", "",
		map[string]any{"username": "newuser", "password": "secret1", "invite_code": code})
	if status != http.StatusCreated {
		t.Fatalf("register with code status = %d body = %v", status, d)
	}

	// The same code cannot be reused.
	status, _ = doJSON(t, http.MethodPost, env.server.URL+"/api/auth/register", "",
		map[string]any{"username": "second", "password": "secret1", "invite_code": code})
	if status != http.StatusForbidden {
		t.Fatalf("reuse code status = %d, want 403", status)
	}

	// The admin list shows the code as used.
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/invites", token, nil)
	for _, it := range d["items"].([]any) {
		m := it.(map[string]any)
		if m["id"] == id && m["used_user"] != "newuser" {
			t.Fatalf("invite not marked used: %v", m)
		}
	}
}
