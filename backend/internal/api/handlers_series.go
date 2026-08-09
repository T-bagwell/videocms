package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

func (a *App) listSeries(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT s.id, s.library_id, l.name, s.name, s.season, s.episode_count, s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND v.available
		        ORDER BY v.season, v.episode LIMIT 1) AS poster_path
		FROM series s
		JOIN libraries l ON l.id = s.library_id
		ORDER BY lower(s.name), s.season`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query series failed")
		return
	}
	defer rows.Close()

	items := []models.Series{}
	for rows.Next() {
		var s models.Series
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
			&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan series failed")
			return
		}
		s.HasPoster = s.PosterPath != ""
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) getSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)

	var s models.Series
	err = a.pool.QueryRow(r.Context(), `
		SELECT s.id, s.library_id, l.name, s.name, s.season, s.episode_count, s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND v.available
		        ORDER BY v.season, v.episode LIMIT 1)
		FROM series s JOIN libraries l ON l.id = s.library_id
		WHERE s.id=$1`, id).Scan(
		&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
		&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "series not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query series failed")
		return
	}
	s.HasPoster = s.PosterPath != ""

	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM videos v
		JOIN libraries l ON l.id = v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		WHERE v.series_id=$2 AND v.available
		ORDER BY v.season, v.episode, lower(v.title)`, videoColumns), user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query episodes failed")
		return
	}
	defer rows.Close()

	items := []models.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan episode failed")
			return
		}
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": s, "items": items})
}

func (a *App) seriesPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(), `
		SELECT (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND v.available
		        ORDER BY v.season, v.episode LIMIT 1)
		FROM series s WHERE s.id=$1`, id).Scan(&path)
	if err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, path)
}
