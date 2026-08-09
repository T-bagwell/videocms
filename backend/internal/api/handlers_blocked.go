package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type blockedTitle struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	MatchCount int64 `json:"match_count"`
	CreatedAt string `json:"created_at"`
}

// blockedTitlesCondition returns a SQL condition matching videos whose title
// contains any blocked title (case-insensitive substring match).
func blockedTitlesCondition() string {
	return `NOT EXISTS (
		SELECT 1 FROM blocked_titles bt
		WHERE position(lower(bt.title) in lower(v.title)) > 0
	)`
}

func (a *App) listBlockedTitles(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT bt.id::text, bt.title, bt.created_at::text,
		       (SELECT count(*) FROM videos v
		        WHERE position(lower(bt.title) in lower(v.title)) > 0) AS match_count
		FROM blocked_titles bt
		ORDER BY bt.created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query blocked titles failed")
		return
	}
	defer rows.Close()

	items := []blockedTitle{}
	for rows.Next() {
		var b blockedTitle
		if err := rows.Scan(&b.ID, &b.Title, &b.CreatedAt, &b.MatchCount); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan blocked title failed")
			return
		}
		items = append(items, b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) createBlockedTitle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	if len([]rune(title)) > 200 {
		writeErr(w, http.StatusBadRequest, "title is too long (max 200 characters)")
		return
	}

	var id string
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO blocked_titles (title) VALUES ($1)
		ON CONFLICT DO NOTHING
		RETURNING id::text`, title).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusOK, map[string]any{"message": "already blocked"})
			return
		}
		writeErr(w, http.StatusInternalServerError, "add blocked title failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "title": title})
}

func (a *App) deleteBlockedTitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid blocked title id")
		return
	}
	tag, err := a.pool.Exec(r.Context(),
		`DELETE FROM blocked_titles WHERE id=$1`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "remove blocked title failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "blocked title not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}
