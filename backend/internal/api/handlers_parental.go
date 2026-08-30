package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"videocms/backend/internal/auth"
)

const unlockTTL = 5 * time.Minute

type unlockClaims struct {
	Unlock string `json:"unlock"`
	jwt.RegisteredClaims
}

func signUnlock(secret string) (string, error) {
	claims := unlockClaims{
		Unlock: "1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(unlockTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func validUnlock(secret, token string) bool {
	if token == "" {
		return false
	}
	claims := &unlockClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	return err == nil && parsed.Valid && claims.Unlock == "1"
}

// PUT /api/users/me/pin — set or clear (empty) the parental PIN.
func (a *App) setPin(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req struct {
		Pin string `json:"pin"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Pin = strings.TrimSpace(req.Pin)
	var hash string
	if req.Pin != "" {
		if len(req.Pin) < 4 || len(req.Pin) > 20 {
			writeErr(w, http.StatusBadRequest, "pin must be 4-20 characters")
			return
		}
		h, err := bcrypt.GenerateFromPassword([]byte(req.Pin), bcrypt.DefaultCost)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "hash pin failed")
			return
		}
		hash = string(h)
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE users SET pin=$1 WHERE id=$2`, hash, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "save pin failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pin_set": req.Pin != ""})
}

// POST /api/users/me/pin/verify — verify the PIN and return a short unlock token.
func (a *App) verifyPin(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req struct {
		Pin string `json:"pin"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var hash string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT pin FROM users WHERE id=$1`, user.ID).Scan(&hash); err != nil || hash == "" {
		writeErr(w, http.StatusBadRequest, "no pin is set")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Pin)) != nil {
		writeErr(w, http.StatusUnauthorized, "wrong pin")
		return
	}
	token, err := signUnlock(a.cfg.JWTSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sign unlock token failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlock_token": token, "expires_in": int(unlockTTL.Seconds())})
}
