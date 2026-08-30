package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

type oidcEndpoints struct {
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	UserInfo      string `json:"userinfo_endpoint"`
}

var oidcStates = struct {
	sync.Mutex
	m map[string]time.Time
}{m: map[string]time.Time{}}

func (a *App) oidcConfigured() bool {
	return a.cfg.OIDCIssuer != "" && a.cfg.OIDCClientID != "" && a.cfg.OIDCRedirectURL != ""
}

func (a *App) oidcDiscover(ctx context.Context) (oidcEndpoints, error) {
	docURL := strings.TrimSuffix(a.cfg.OIDCIssuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docURL, nil)
	if err != nil {
		return oidcEndpoints{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return oidcEndpoints{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return oidcEndpoints{}, fmt.Errorf("oidc discovery: HTTP %d", res.StatusCode)
	}
	var ep oidcEndpoints
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&ep); err != nil {
		return oidcEndpoints{}, err
	}
	return ep, nil
}

// GET /api/auth/oidc/start — redirect to the IdP authorization endpoint.
func (a *App) oidcStart(w http.ResponseWriter, r *http.Request) {
	if !a.oidcConfigured() {
		writeErr(w, http.StatusBadRequest, "OIDC is not configured (set OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_REDIRECT_URL)")
		return
	}
	ep, err := a.oidcDiscover(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "oidc discovery failed: "+err.Error())
		return
	}
	state := make([]byte, 16)
	_, _ = rand.Read(state)
	stateHex := hex.EncodeToString(state)
	oidcStates.Lock()
	oidcStates.m[stateHex] = time.Now().Add(5 * time.Minute)
	oidcStates.Unlock()

	params := url.Values{
		"client_id":     {a.cfg.OIDCClientID},
		"redirect_uri":  {a.cfg.OIDCRedirectURL},
		"response_type": {"code"},
		"scope":         {"openid profile email"},
		"state":         {stateHex},
	}
	http.Redirect(w, r, ep.Authorization+"?"+params.Encode(), http.StatusFound)
}

// GET /api/auth/oidc/callback — exchange the code, resolve the user and
// redirect to the frontend with a session token.
func (a *App) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if !a.oidcConfigured() {
		writeErr(w, http.StatusBadRequest, "OIDC is not configured")
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	oidcStates.Lock()
	exp, ok := oidcStates.m[state]
	if ok {
		delete(oidcStates.m, state)
	}
	oidcStates.Unlock()
	if !ok || time.Now().After(exp) || code == "" {
		writeErr(w, http.StatusBadRequest, "invalid OIDC state or code")
		return
	}
	ep, err := a.oidcDiscover(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "oidc discovery failed")
		return
	}

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {a.cfg.OIDCRedirectURL},
		"client_id":    {a.cfg.OIDCClientID},
	}
	if a.cfg.OIDCClientSecret != "" {
		form.Set("client_secret", a.cfg.OIDCClientSecret)
	}
	tokenReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ep.Token, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build token request failed")
		return
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRes, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "oidc token exchange failed")
		return
	}
	defer func() { _ = tokenRes.Body.Close() }()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if tokenRes.StatusCode < 200 || tokenRes.StatusCode >= 300 ||
		json.NewDecoder(io.LimitReader(tokenRes.Body, 1<<20)).Decode(&tok) != nil ||
		tok.AccessToken == "" {
		writeErr(w, http.StatusBadGateway, "oidc token exchange failed")
		return
	}

	uiReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, ep.UserInfo, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build userinfo request failed")
		return
	}
	uiReq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	uiRes, err := http.DefaultClient.Do(uiReq)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "oidc userinfo failed")
		return
	}
	defer func() { _ = uiRes.Body.Close() }()
	var info struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if uiRes.StatusCode < 200 || uiRes.StatusCode >= 300 ||
		json.NewDecoder(io.LimitReader(uiRes.Body, 1<<20)).Decode(&info) != nil ||
		info.Sub == "" {
		writeErr(w, http.StatusBadGateway, "oidc userinfo failed")
		return
	}

	user, err := a.upsertOIDCUser(r.Context(), info.Sub, info.Email, info.Name, info.PreferredUsername)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "resolve OIDC user failed")
		return
	}
	jwt, err := auth.Sign(a.cfg.JWTSecret, user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sign session failed")
		return
	}
	http.Redirect(w, r, "/login?sso_token="+url.QueryEscape(jwt), http.StatusFound)
}

func (a *App) upsertOIDCUser(ctx context.Context, sub, email, name, preferred string) (models.User, error) {
	user := models.User{}
	err := a.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, display_name, role, created_at
		FROM users WHERE oauth_sub=$1`, sub).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Role, &user.CreatedAt)
	if err == nil {
		return user, nil
	}
	username := email
	if username == "" {
		username = preferred
	}
	if username == "" {
		username = "user-" + sub
	}
	if len(username) > 60 {
		username = username[:60]
	}
	display := name
	if display == "" {
		display = username
	}
	// Try linking to an existing account with the same username.
	linked := false
	err = a.pool.QueryRow(ctx, `
		UPDATE users SET oauth_sub=$1 WHERE username=$2 AND oauth_sub IS NULL
		RETURNING id, username, password_hash, display_name, role, created_at`,
		sub, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Role, &user.CreatedAt)
	if err == nil {
		linked = true
	}
	if !linked {
		random := make([]byte, 24)
		_, _ = rand.Read(random)
		hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(random)), bcrypt.DefaultCost)
		if err != nil {
			return user, err
		}
		insertErr := a.pool.QueryRow(ctx, `
			INSERT INTO users (username, password_hash, display_name, role, oauth_sub)
			VALUES ($1, $2, $3, 'user', $4)
			RETURNING id, username, password_hash, display_name, role, created_at`,
			username, string(hash), display, sub).
			Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Role, &user.CreatedAt)
		if insertErr != nil {
			return user, insertErr
		}
	}
	return user, nil
}
