package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"videocms/backend/internal/models"
)

type ctxKey int

const userKey ctxKey = 0

func UserFrom(r *http.Request) *models.User {
	u, _ := r.Context().Value(userKey).(*models.User)
	return u
}

func RequireAuth(pool *pgxpool.Pool, secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		tokenString := ""
		if strings.HasPrefix(header, "Bearer ") {
			tokenString = strings.TrimPrefix(header, "Bearer ")
		} else if t := r.URL.Query().Get("token"); t != "" {
			// media tags (<video>/<img>) cannot set headers; allow token query param
			tokenString = t
		}
		if tokenString == "" {
			writeAuthError(w, "missing bearer token")
			return
		}
		claims, err := Parse(secret, tokenString)
		if err != nil {
			writeAuthError(w, "invalid or expired token")
			return
		}

		user := &models.User{}
		err = pool.QueryRow(r.Context(),
			`SELECT id, username, password_hash, display_name, role, created_at
			 FROM users WHERE id=$1`, claims.UserID,
		).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName, &user.Role, &user.CreatedAt)
		if err != nil {
			writeAuthError(w, "user not found")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))
	}
}

func RequireAdmin(pool *pgxpool.Pool, secret string, next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(pool, secret, func(w http.ResponseWriter, r *http.Request) {
		user := UserFrom(r)
		if user.Role != models.RoleAdmin {
			writeAuthError(w, "admin privileges required")
			return
		}
		next(w, r)
	})
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	b, _ := json.Marshal(map[string]string{"error": msg})
	_, _ = w.Write(b)
}
