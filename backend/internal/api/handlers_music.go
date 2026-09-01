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

const albumColumns = `
	a.id, a.library_id, l.name, a.name, a.artist, a.year,
	(SELECT count(*) FROM videos v WHERE v.album_id=a.id AND v.available) AS track_count,
	a.cover_path, a.updated_at`

func scanAlbum(row pgx.Row) (models.Album, error) {
	var al models.Album
	err := row.Scan(&al.ID, &al.LibraryID, &al.LibraryName, &al.Name, &al.Artist,
		&al.Year, &al.TrackCount, &al.CoverPath, &al.UpdatedAt)
	al.HasCover = al.CoverPath != ""
	return al, err
}

// GET /api/albums lists music albums that have at least one visible track.
func (a *App) listAlbums(w http.ResponseWriter, r *http.Request) {
	args := []any{auth.UserFrom(r).ID}
	where := fmt.Sprintf(`EXISTS (SELECT 1 FROM videos v WHERE v.album_id=a.id AND %s)`, visibleEpisodes(1))
	if libID := r.URL.Query().Get("library_id"); libID != "" {
		id, err := uuid.Parse(libID)
		if err == nil {
			where += fmt.Sprintf(` AND a.library_id=$%d`, len(args)+1)
			args = append(args, id)
		}
	}
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM albums a JOIN libraries l ON l.id=a.library_id
		WHERE %s ORDER BY lower(a.artist), lower(a.name)`, albumColumns, where), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query albums failed")
		return
	}
	defer rows.Close()
	items := []models.Album{}
	for rows.Next() {
		al, err := scanAlbum(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan album failed")
			return
		}
		items = append(items, al)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/albums/{id} returns the album and its visible tracks.
func (a *App) getAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid album id")
		return
	}
	al, err := scanAlbum(a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT %s FROM albums a JOIN libraries l ON l.id=a.library_id
		WHERE a.id=$1 AND EXISTS (SELECT 1 FROM videos v WHERE v.album_id=a.id AND %s)`,
		albumColumns, visibleEpisodes(2)), id, auth.UserFrom(r).ID))
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "album not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query album failed")
		return
	}
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT v.id, v.title, v.filename, v.artist, v.album, v.duration_sec,
		       v.size_bytes, v.year, v.poster_path
		FROM videos v WHERE v.album_id=$1 AND %s
		ORDER BY lower(v.title), v.filename`, visibleEpisodes(2)), id, auth.UserFrom(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query album tracks failed")
		return
	}
	defer rows.Close()
	tracks := []models.MusicTrack{}
	for rows.Next() {
		var t models.MusicTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.Filename, &t.Artist, &t.Album,
			&t.DurationSec, &t.SizeBytes, &t.Year, &t.PosterPath); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan album track failed")
			return
		}
		t.HasPoster = t.PosterPath != ""
		tracks = append(tracks, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"album": al, "tracks": tracks})
}

// GET /api/albums/{id}/poster
func (a *App) albumPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid album id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(),
		`SELECT cover_path FROM albums WHERE id=$1`, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) || path == "" {
		writeErr(w, http.StatusNotFound, "album cover not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query album cover failed")
		return
	}
	http.ServeFile(w, r, path)
}
