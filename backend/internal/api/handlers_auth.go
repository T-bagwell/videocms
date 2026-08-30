package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

type registerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(req.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 3-32 letters, digits or underscores")
		return
	}
	if len(req.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := models.User{Username: req.Username, DisplayName: req.DisplayName, Role: models.RoleUser}
	err = a.pool.QueryRow(r.Context(),
		`INSERT INTO users (username, password_hash, display_name, role)
		 VALUES ($1,$2,$3,'user')
		 RETURNING id, created_at`,
		req.Username, string(hash), req.DisplayName,
	).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeErr(w, http.StatusConflict, "username already taken")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := auth.Sign(a.cfg.JWTSecret, user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": user})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := models.User{}
	err := a.pool.QueryRow(r.Context(),
		`SELECT id, username, password_hash, display_name, role, created_at
		 FROM users WHERE username=$1`, strings.TrimSpace(req.Username),
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Role, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "login failed")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, err := auth.Sign(a.cfg.JWTSecret, user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, auth.UserFrom(r))
}

// GET /api/auth/sso — which single sign-on providers are configured.
func (a *App) ssoStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"oidc": a.oidcConfigured(),
		"saml": a.samlConfigured(),
	})
}
