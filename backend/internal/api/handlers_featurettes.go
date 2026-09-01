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

type featurette struct {
	ID        uuid.UUID `json:"id"`
	VideoID   uuid.UUID `json:"video_id"`
	Title     string    `json:"title"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

var featuretteExts = map[string]bool{
	".mp4": true, ".m4v": true, ".webm": true, ".mkv": true,
	".mov": true, ".avi": true, ".mpg": true, ".mpeg": true,
}

// GET /api/videos/{id}/featurettes
func (a *App) listFeaturettes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	rows, err := a.pool.Query(r.Context(),
		`SELECT id, video_id, title, size_bytes, created_at
		 FROM featurettes WHERE video_id=$1 ORDER BY created_at, id`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query featurettes failed")
		return
	}
	defer rows.Close()
	items := []featurette{}
	for rows.Next() {
		var f featurette
		if err := rows.Scan(&f.ID, &f.VideoID, &f.Title, &f.SizeBytes, &f.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan featurette failed")
			return
		}
		items = append(items, f)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/videos/{id}/featurettes stores an uploaded featurette file next to
// the movie (under DATA_DIR/featurettes/<video_id>/).
func (a *App) addFeaturette(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, 4<<30)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if !featuretteExts[ext] {
		writeErr(w, http.StatusBadRequest, "unsupported featurette format")
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	if len(title) > 200 {
		title = title[:200]
	}

	dir := filepath.Join(a.cfg.DataDir, "featurettes", id.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "create featurette dir failed")
		return
	}
	dst := filepath.Join(dir, uuid.NewString()+ext)
	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create featurette file failed")
		return
	}
	size, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dst)
		writeErr(w, http.StatusInternalServerError, "save featurette failed")
		return
	}

	var f featurette
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO featurettes (video_id, title, file_path, size_bytes)
		VALUES ($1,$2,$3,$4) RETURNING id, video_id, title, size_bytes, created_at`,
		id, title, dst, size).Scan(&f.ID, &f.VideoID, &f.Title, &f.SizeBytes, &f.CreatedAt); err != nil {
		_ = os.Remove(dst)
		log.Printf("insert featurette: %v", err)
		writeErr(w, http.StatusInternalServerError, "save featurette record failed")
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// DELETE /api/videos/{id}/featurettes/{fid}
func (a *App) deleteFeaturette(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	fid, err := uuid.Parse(r.PathValue("fid"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid featurette id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM featurettes WHERE id=$1 AND video_id=$2`, fid, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "featurette not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query featurette failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM featurettes WHERE id=$1`, fid); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete featurette record failed")
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("remove featurette file %s: %v", path, err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// GET /api/videos/{id}/featurettes/{fid}/stream
func (a *App) streamFeaturette(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	fid, err := uuid.Parse(r.PathValue("fid"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid featurette id")
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
		`SELECT file_path FROM featurettes WHERE id=$1 AND video_id=$2`, fid, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "featurette not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query featurette failed")
		return
	}
	http.ServeFile(w, r, path)
}
