package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

// GET /api/videos/{id}/similar — recommend up to 8 related videos by shared
// genres, year, series and tags.
func (a *App) similarVideos(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	query := fmt.Sprintf(`
		WITH src AS (SELECT id, year, series_id, genres FROM videos WHERE id=$2)
		SELECT %s
		FROM videos v, src
		WHERE v.id <> src.id AND %s
		ORDER BY
			(SELECT count(*) FROM unnest(v.genres) g WHERE g = ANY(src.genres))
			+ CASE WHEN v.year = src.year THEN 1 ELSE 0 END
			+ CASE WHEN v.series_id IS NOT NULL AND v.series_id = src.series_id THEN 2 ELSE 0 END
			+ (SELECT count(*) FROM video_tags vt1 JOIN video_tags vt2 ON vt1.tag_id = vt2.tag_id
			   WHERE vt1.video_id = v.id AND vt2.video_id = src.id)
			DESC, v.created_at DESC
		LIMIT 8`,
		videoColumns, visibleEpisodes(1))
	rows, err := a.pool.Query(r.Context(), query, user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "similar videos failed")
		return
	}
	defer rows.Close()
	items := []models.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			continue
		}
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type tagCount struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Kind  string    `json:"kind"`
	Count int       `json:"count"`
}

// GET /api/tags — tag cloud with usage counts.
func (a *App) tagCloud(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT t.id, t.name, t.kind, count(vt.video_id)
		FROM tags t JOIN video_tags vt ON vt.tag_id = t.id
		GROUP BY t.id
		ORDER BY count(vt.video_id) DESC, t.name
		LIMIT 100`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "tag cloud failed")
		return
	}
	defer rows.Close()
	items := []tagCount{}
	for rows.Next() {
		var t tagCount
		if err := rows.Scan(&t.ID, &t.Name, &t.Kind, &t.Count); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan tag failed")
			return
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
