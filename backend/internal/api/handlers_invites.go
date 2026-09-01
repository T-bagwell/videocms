package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type inviteCode struct {
	ID        uuid.UUID  `json:"id"`
	Code      string     `json:"code"`
	UsedBy    *uuid.UUID `json:"used_by,omitempty"`
	UsedUser  string     `json:"used_user"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// GET /api/admin/invites
func (a *App) listInvites(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT ic.id, ic.code, ic.used_by, COALESCE(u.username, ''), ic.used_at, ic.created_at
		FROM invite_codes ic LEFT JOIN users u ON u.id = ic.used_by
		ORDER BY ic.created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query invites failed")
		return
	}
	defer rows.Close()
	items := []inviteCode{}
	for rows.Next() {
		var it inviteCode
		if err := rows.Scan(&it.ID, &it.Code, &it.UsedBy, &it.UsedUser, &it.UsedAt, &it.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan invite failed")
			return
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/invites generates one-time invite codes.
func (a *App) generateInvites(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count int `json:"count"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 50 {
		req.Count = 50
	}
	created := 0
	for i := 0; i < req.Count; i++ {
		buf := make([]byte, 6)
		if _, err := rand.Read(buf); err != nil {
			writeErr(w, http.StatusInternalServerError, "generate invite failed")
			return
		}
		code := hex.EncodeToString(buf)
		if _, err := a.pool.Exec(r.Context(),
			`INSERT INTO invite_codes (code) VALUES ($1) ON CONFLICT (code) DO NOTHING`,
			code); err == nil {
			created++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": created})
}

// DELETE /api/admin/invites/{id}
func (a *App) revokeInvite(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid invite id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM invite_codes WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete invite failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}
