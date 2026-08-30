package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

type adminUser struct {
	ID          uuid.UUID   `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	Role        models.Role `json:"role"`
	CreatedAt   time.Time   `json:"created_at"`
}

func (a *App) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, username, display_name, role, created_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query users failed")
		return
	}
	defer rows.Close()

	users := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan user failed")
			return
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

type updateUserRequest struct {
	Role          *models.Role `json:"role"`
	DisplayName   *string      `json:"display_name"`
	AllowedRating *string      `json:"allowed_rating"`
}

func (a *App) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req updateUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Role != nil {
		role := *req.Role
		if role != models.RoleUser && role != models.RoleAdmin {
			writeErr(w, http.StatusBadRequest, "role must be 'user' or 'admin'")
			return
		}
		var isAdmin bool
		if err := a.pool.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin')`, id).Scan(&isAdmin); err != nil {
			writeErr(w, http.StatusInternalServerError, "check user failed")
			return
		}
		if isAdmin && role == models.RoleUser {
			var adminCount int
			if err := a.pool.QueryRow(r.Context(),
				`SELECT count(*) FROM users WHERE role='admin'`).Scan(&adminCount); err != nil {
				writeErr(w, http.StatusInternalServerError, "count admins failed")
				return
			}
			if adminCount <= 1 {
				writeErr(w, http.StatusConflict, "cannot demote the last admin")
				return
			}
		}
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET role=$1 WHERE id=$2`, role, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update role failed")
			return
		}
	}
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "display_name cannot be empty")
			return
		}
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET display_name=$1 WHERE id=$2`, name, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update display name failed")
			return
		}
	}
	if req.AllowedRating != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET allowed_rating=$1 WHERE id=$2`, strings.TrimSpace(*req.AllowedRating), id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update allowed rating failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

func (a *App) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Password) < 6 {
		writeErr(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash failed")
		return
	}
	tag, err := a.pool.Exec(r.Context(),
		`UPDATE users SET password_hash=$1 WHERE id=$2`, string(hash), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update password failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "password reset"})
}

func (a *App) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	me := auth.UserFrom(r)
	if me.ID == id {
		writeErr(w, http.StatusConflict, "cannot delete your own account")
		return
	}

	var role models.Role
	err = a.pool.QueryRow(r.Context(),
		`SELECT role FROM users WHERE id=$1`, id).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load user failed")
		return
	}
	if role == models.RoleAdmin {
		var adminCount int
		if err := a.pool.QueryRow(r.Context(),
			`SELECT count(*) FROM users WHERE role='admin'`).Scan(&adminCount); err != nil {
			writeErr(w, http.StatusInternalServerError, "count admins failed")
			return
		}
		if adminCount <= 1 {
			writeErr(w, http.StatusConflict, "cannot delete the last admin")
			return
		}
	}
	if _, err := a.pool.Exec(r.Context(), `DELETE FROM users WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete user failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "user deleted"})
}
