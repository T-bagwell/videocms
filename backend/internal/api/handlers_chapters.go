package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type chapter struct {
	ID       uuid.UUID `json:"id"`
	Position int       `json:"position"`
	StartSec float64   `json:"start_sec"`
	EndSec   float64   `json:"end_sec"`
	Title    string    `json:"title"`
}

// GET /api/videos/{id}/chapters returns the chapter index parsed from
// ffprobe, ordered by start time. The same shape serves as the generic
// media-segment index for the player timeline.
func (a *App) listChapters(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	userID := auth.UserFrom(r).ID
	var visible bool
	if err := a.pool.QueryRow(r.Context(), fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM videos v WHERE v.id=$1 AND %s)`,
		visibleEpisodes(2)), id, userID).Scan(&visible); err != nil || !visible {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	rows, err := a.pool.Query(r.Context(),
		`SELECT id, position, start_sec, end_sec, title
		 FROM chapters WHERE video_id=$1 ORDER BY position, start_sec`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query chapters failed")
		return
	}
	defer rows.Close()
	items := []chapter{}
	for rows.Next() {
		var c chapter
		if err := rows.Scan(&c.ID, &c.Position, &c.StartSec, &c.EndSec, &c.Title); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan chapter failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
