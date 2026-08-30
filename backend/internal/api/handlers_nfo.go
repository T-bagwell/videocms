package api

import (
	"net/http"
	"os"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
	"videocms/backend/internal/models"
)

// POST /api/libraries/{id}/export-nfo — write an NFO next to every video.
func (a *App) exportLibraryNFO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, title, year, synopsis, genres, file_path
		FROM videos WHERE library_id=$1 AND available`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "export nfo failed")
		return
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var v struct {
			Title    string
			Year     int
			Synopsis string
			Genres   []string
			Path     string
		}
		if err := rows.Scan(&v.Title, &v.Year, &v.Synopsis, &v.Genres, &v.Path); err != nil {
			continue
		}
		mv := models.Video{Title: v.Title, Year: v.Year, Synopsis: v.Synopsis, Genres: v.Genres, FilePath: v.Path}
		if _, err := media.WriteNFO(mv); err == nil {
			count++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"exported": count})
}

// POST /api/libraries/{id}/import-nfo — apply NFO metadata next to videos.
func (a *App) importLibraryNFO(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, file_path FROM videos WHERE library_id=$1 AND available`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "import nfo failed")
		return
	}
	defer rows.Close()
	updated := 0
	for rows.Next() {
		var videoID uuid.UUID
		var path string
		if err := rows.Scan(&videoID, &path); err != nil {
			continue
		}
		nfoPath := media.NFOFileFor(path)
		if _, err := os.Stat(nfoPath); err != nil {
			continue
		}
		d, err := media.ReadNFO(nfoPath)
		if err != nil || d.Title == "" {
			continue
		}
		if _, err := a.pool.Exec(r.Context(), `
			UPDATE videos SET title=$2, year=$3, synopsis=$4, genres=$5, updated_at=now()
			WHERE id=$1`,
			videoID, d.Title, d.Year, d.Synopsis, d.Genres); err != nil {
			continue
		}
		updated++
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}
