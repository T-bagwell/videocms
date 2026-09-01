package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
)

type contentRequest struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Username  string     `json:"username"`
	Title     string     `json:"title"`
	Year      int        `json:"year"`
	MediaType string     `json:"media_type"`
	Notes     string     `json:"notes"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

const requestColumns = `
	r.id, r.user_id, COALESCE(u.username, ''), r.title, r.year, r.media_type,
	r.notes, r.status, r.created_at, r.decided_at`

func scanContentRequest(row pgx.Row) (contentRequest, error) {
	var c contentRequest
	err := row.Scan(&c.ID, &c.UserID, &c.Username, &c.Title, &c.Year, &c.MediaType,
		&c.Notes, &c.Status, &c.CreatedAt, &c.DecidedAt)
	return c, err
}

// POST /api/requests
func (a *App) createContentRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string `json:"title"`
		Year      int    `json:"year"`
		MediaType string `json:"media_type"`
		Notes     string `json:"notes"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.MediaType == "" {
		req.MediaType = "movie"
	}
	if len(req.Title) > 200 {
		req.Title = req.Title[:200]
	}
	if len(req.Notes) > 2000 {
		req.Notes = req.Notes[:2000]
	}
	var c contentRequest
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO requests (user_id, title, year, media_type, notes)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, user_id, '', title, year, media_type, notes, status, created_at, NULL`,
		auth.UserFrom(r).ID, req.Title, req.Year, req.MediaType, strings.TrimSpace(req.Notes)).
		Scan(&c.ID, &c.UserID, &c.Username, &c.Title, &c.Year, &c.MediaType,
			&c.Notes, &c.Status, &c.CreatedAt, &c.DecidedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create request failed")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// GET /api/requests — the current user's requests.
func (a *App) listMyRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM requests r JOIN users u ON u.id=r.user_id
		WHERE r.user_id=$1 ORDER BY r.created_at DESC`, requestColumns), auth.UserFrom(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query requests failed")
		return
	}
	defer rows.Close()
	items := []contentRequest{}
	for rows.Next() {
		c, err := scanContentRequest(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan request failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/requests/all — admin queue.
func (a *App) listAllRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM requests r JOIN users u ON u.id=r.user_id
		ORDER BY (r.status='pending') DESC, r.created_at DESC`, requestColumns))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query requests failed")
		return
	}
	defer rows.Close()
	items := []contentRequest{}
	for rows.Next() {
		c, err := scanContentRequest(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan request failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/requests/{id}/decide — admin approves (optionally queuing a
// yt-dlp download) or rejects.
func (a *App) decideContentRequest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request id")
		return
	}
	var req struct {
		Status      string `json:"status"`
		DownloadURL string `json:"download_url"`
		TargetPath  string `json:"target_path"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status != "approved" && req.Status != "rejected" {
		writeErr(w, http.StatusBadRequest, "status must be approved or rejected")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM requests WHERE id=$1)`, id).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "request not found")
		return
	}
	status := req.Status
	if req.Status == "approved" && strings.TrimSpace(req.DownloadURL) != "" {
		dlURL := strings.TrimSpace(req.DownloadURL)
		if !strings.HasPrefix(dlURL, "http://") && !strings.HasPrefix(dlURL, "https://") {
			writeErr(w, http.StatusBadRequest, "download_url must start with http:// or https://")
			return
		}
		target := strings.TrimSpace(req.TargetPath)
		if target != "" && !isAbsDir(target) {
			writeErr(w, http.StatusBadRequest, "target_path must be an existing absolute directory")
			return
		}
		if _, err := a.pool.Exec(r.Context(), `
			INSERT INTO downloads (url, target_path, format)
			VALUES ($1,$2,$3)`,
			dlURL, target, defaultYtDlpFormat); err != nil {
			writeErr(w, http.StatusInternalServerError, "queue download failed")
			return
		}
		status = "downloading"
	}
	admin := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(), `
		UPDATE requests SET status=$1, decided_by=$2, decided_at=now() WHERE id=$3`,
		status, admin.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update request failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated", "status": status})
}
