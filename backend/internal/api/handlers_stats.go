package api

import (
	"net/http"

	"videocms/backend/internal/models"
)

func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	var s models.Stats
	if err := a.pool.QueryRow(r.Context(), `
		SELECT
			(SELECT count(*) FROM users),
			(SELECT count(*) FROM libraries),
			(SELECT count(*) FROM videos),
			(SELECT count(*) FROM videos WHERE available=false),
			(SELECT count(*) FROM playlists),
			(SELECT count(*) FROM favorites),
			(SELECT COALESCE(sum(size_bytes),0) FROM videos)`).Scan(
		&s.Users, &s.Libraries, &s.Videos, &s.VideosMissing, &s.Playlists, &s.Favorites, &s.TotalBytes); err != nil {
		writeErr(w, http.StatusInternalServerError, "query stats failed")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

