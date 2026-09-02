package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type reportItem struct {
	ID         uuid.UUID  `json:"id"`
	VideoID    uuid.UUID  `json:"video_id"`
	VideoTitle string     `json:"video_title"`
	Reporter   string     `json:"reporter"`
	Reason     string     `json:"reason"`
	Details    string     `json:"details"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	DecidedAt  *time.Time `json:"decided_at,omitempty"`
}

// POST /api/videos/{id}/report
func (a *App) reportVideo(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeErr(w, http.StatusBadRequest, "reason is required")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM reports WHERE video_id=$1 AND user_id=$2 AND status='pending')`,
		id, user.ID).Scan(&exists); err != nil || exists {
		writeErr(w, http.StatusConflict, "you already reported this video")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO reports (video_id, user_id, reason, details)
		VALUES ($1,$2,$3,$4)`,
		id, user.ID, req.Reason, strings.TrimSpace(req.Details)); err != nil {
		writeErr(w, http.StatusInternalServerError, "submit report failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "reported"})
}

// GET /api/admin/reports
func (a *App) listReports(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT rep.id, rep.video_id, COALESCE(v.title, ''), COALESCE(u.username, ''),
		       rep.reason, rep.details, rep.status, rep.created_at, rep.decided_at
		FROM reports rep
		LEFT JOIN videos v ON v.id = rep.video_id
		LEFT JOIN users u ON u.id = rep.user_id
		ORDER BY (rep.status='pending') DESC, rep.created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query reports failed")
		return
	}
	defer rows.Close()
	items := []reportItem{}
	for rows.Next() {
		var it reportItem
		if err := rows.Scan(&it.ID, &it.VideoID, &it.VideoTitle, &it.Reporter,
			&it.Reason, &it.Details, &it.Status, &it.CreatedAt, &it.DecidedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan report failed")
			return
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/reports/{id}/decide
func (a *App) decideReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid report id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status != "reviewed" && req.Status != "dismissed" {
		writeErr(w, http.StatusBadRequest, "status must be reviewed or dismissed")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		UPDATE reports SET status=$1, decided_by=$2, decided_at=now() WHERE id=$3`,
		req.Status, auth.UserFrom(r).ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update report failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

// PATCH /api/admin/users/{id}/moderation
func (a *App) moderationUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req struct {
		Muted         *bool `json:"muted"`
		GlobalBlocked *bool `json:"global_blocked"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Muted == nil && req.GlobalBlocked == nil {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if req.Muted != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET muted=$1 WHERE id=$2`, *req.Muted, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update user failed")
			return
		}
	}
	if req.GlobalBlocked != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET global_blocked=$1 WHERE id=$2`, *req.GlobalBlocked, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update user failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

// POST /api/admin/users/bulk applies one action to many users.
func (a *App) bulkUserAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []uuid.UUID `json:"ids"`
		Action string      `json:"action"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, "ids are required")
		return
	}
	var set string
	switch req.Action {
	case "mute":
		set = "muted=true"
	case "unmute":
		set = "muted=false"
	case "block":
		set = "global_blocked=true"
	case "unblock":
		set = "global_blocked=false"
	default:
		writeErr(w, http.StatusBadRequest, "unknown action")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE users SET `+set+` WHERE id = ANY($1::uuid[])`, req.IDs); err != nil {
		writeErr(w, http.StatusInternalServerError, "bulk update failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}
