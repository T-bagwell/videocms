package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
)

type themeSong struct {
	ID        uuid.UUID `json:"id"`
	VideoID   uuid.UUID `json:"video_id"`
	Title     string    `json:"title"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

var themeSongExts = map[string]bool{
	".mp3": true, ".m4a": true, ".aac": true, ".ogg": true,
	".opus": true, ".flac": true, ".wav": true,
}

// GET /api/videos/{id}/theme-song
func (a *App) getThemeSong(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var s themeSong
	err = a.pool.QueryRow(r.Context(),
		`SELECT id, video_id, title, size_bytes, created_at
		 FROM theme_songs WHERE video_id=$1`, id).
		Scan(&s.ID, &s.VideoID, &s.Title, &s.SizeBytes, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no theme song")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query theme song failed")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// POST /api/videos/{id}/theme-song stores (or replaces) the theme song file.
func (a *App) upsertThemeSong(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM videos WHERE id=$1)`, id).Scan(&exists); err != nil {
		writeErr(w, http.StatusInternalServerError, "check video failed")
		return
	}
	if !exists {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if !themeSongExts[ext] {
		writeErr(w, http.StatusBadRequest, "unsupported theme song format")
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	if len(title) > 200 {
		title = title[:200]
	}

	dir := filepath.Join(a.cfg.DataDir, "theme-songs", id.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "create theme song dir failed")
		return
	}
	dst := filepath.Join(dir, uuid.NewString()+ext)
	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create theme song file failed")
		return
	}
	size, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dst)
		writeErr(w, http.StatusInternalServerError, "save theme song failed")
		return
	}

	var oldPath string
	_ = a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM theme_songs WHERE video_id=$1`, id).Scan(&oldPath)
	var s themeSong
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO theme_songs (video_id, title, file_path, size_bytes)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (video_id) DO UPDATE SET
			title=EXCLUDED.title, file_path=EXCLUDED.file_path,
			size_bytes=EXCLUDED.size_bytes, created_at=now()
		RETURNING id, video_id, title, size_bytes, created_at`,
		id, title, dst, size).Scan(&s.ID, &s.VideoID, &s.Title, &s.SizeBytes, &s.CreatedAt); err != nil {
		_ = os.Remove(dst)
		log.Printf("upsert theme song: %v", err)
		writeErr(w, http.StatusInternalServerError, "save theme song record failed")
		return
	}
	if oldPath != "" && oldPath != dst {
		_ = os.Remove(oldPath)
	}
	writeJSON(w, http.StatusCreated, s)
}

// DELETE /api/videos/{id}/theme-song
func (a *App) deleteThemeSong(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM theme_songs WHERE video_id=$1`, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no theme song")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query theme song failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM theme_songs WHERE video_id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete theme song record failed")
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("remove theme song file %s: %v", path, err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// GET /api/videos/{id}/theme-song/stream
func (a *App) streamThemeSong(w http.ResponseWriter, r *http.Request) {
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
	var path string
	err = a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM theme_songs WHERE video_id=$1`, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no theme song")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query theme song failed")
		return
	}
	http.ServeFile(w, r, path)
}
