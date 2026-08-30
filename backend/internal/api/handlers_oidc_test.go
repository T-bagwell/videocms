package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOIDCLogin(t *testing.T) {
	env := newIntegrationEnv(t)
	var base string
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "openid-configuration"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 "https://idp.example",
				"authorization_endpoint": base + "/auth",
				"token_endpoint":         base + "/token",
				"userinfo_endpoint":      base + "/userinfo",
			})
		case r.URL.Path == "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok123"})
		case r.URL.Path == "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"sub": "abc123", "email": "sso@example.com", "name": "SSO User",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()
	base = idp.URL
	issuer := strings.TrimSuffix(idp.URL, "/") + "/realms/test" // discovery path only used for the doc URL
	env.app.cfg.OIDCIssuer = issuer
	env.app.cfg.OIDCClientID = "client"
	env.app.cfg.OIDCRedirectURL = "http://localhost/cb"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("stop")
		},
	}
	res, err := client.Get(env.server.URL + "/api/auth/oidc/start")
	if err == nil {
		t.Fatal("expected redirect to be intercepted")
	}
	// The redirect error carries the Location in the request URL.
	var loc string
	if res != nil {
		loc = res.Header.Get("Location")
	}
	if loc == "" {
		// fall back: extract from the error's request
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.URL != "" {
			loc = urlErr.URL
		}
	}
	if !strings.Contains(loc, "response_type=code") {
		t.Fatalf("start redirect missing code params: %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("start redirect missing state")
	}

	res, err = client.Get(env.server.URL + "/api/auth/oidc/callback?state=" + state + "&code=xyz")
	if err == nil {
		t.Fatal("expected callback redirect to be intercepted")
	}
	callbackLoc := ""
	if res != nil {
		callbackLoc = res.Header.Get("Location")
	}
	if callbackLoc == "" {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.URL != "" {
			callbackLoc = urlErr.URL
		}
	}
	if !strings.Contains(callbackLoc, "sso_token=") {
		t.Fatalf("callback redirect missing sso_token: %q", callbackLoc)
	}
	cu, err := url.Parse(callbackLoc)
	if err != nil {
		t.Fatal(err)
	}
	ssoToken := cu.Query().Get("sso_token")
	if ssoToken == "" {
		t.Fatal("callback missing sso_token")
	}

	req, _ := http.NewRequest("GET", env.server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+ssoToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("me status = %d: %s", resp.StatusCode, b)
	}
	var me struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&me)
	if me.Username != "sso@example.com" {
		t.Fatalf("me username = %q", me.Username)
	}
}
