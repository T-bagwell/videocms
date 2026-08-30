package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

// thumbnailMu serializes on-demand thumbnail generation per server process so
// concurrent preview requests do not run the same ffmpeg extraction twice.
var thumbnailMu sync.Mutex

func (a *App) thumbnailsDir(id uuid.UUID) string {
	return filepath.Join(a.cfg.DataDir, "thumbnails", id.String())
}

// ensureThumbnails generates the frame set on first request and returns its
// metadata. A missing video or generation failure returns ok=false.
func (a *App) ensureThumbnails(r *http.Request, id uuid.UUID) (media.ThumbnailsMeta, bool) {
	var path string
	var duration float64
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path, duration_sec FROM videos WHERE id=$1 AND available`, id).
		Scan(&path, &duration); err != nil {
		return media.ThumbnailsMeta{}, false
	}
	dir := a.thumbnailsDir(id)
	thumbnailMu.Lock()
	defer thumbnailMu.Unlock()
	if media.CountThumbnails(dir) == 0 {
		meta, err := media.GenerateThumbnails(r.Context(), media.ResolveTool("ffmpeg"), path, duration, dir)
		if err != nil {
			log.Printf("thumbnails for %s: %v", id.String()[:8], err)
			return media.ThumbnailsMeta{}, false
		}
		return meta, true
	}
	return media.ThumbnailsMeta{
		Count:       media.CountThumbnails(dir),
		IntervalSec: 10,
		Width:       160,
		Height:      90,
	}, true
}

// GET /api/videos/{id}/thumbnails — trick-play frame metadata.
func (a *App) videoThumbnails(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	meta, ok := a.ensureThumbnails(r, id)
	if !ok {
		writeErr(w, http.StatusNotFound, "thumbnails unavailable for this video")
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// GET /api/videos/{id}/thumbnails/{n} — one preview frame (1-based).
func (a *App) videoThumbnailImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 {
		writeErr(w, http.StatusBadRequest, "invalid thumbnail index")
		return
	}
	dir := a.thumbnailsDir(id)
	if media.CountThumbnails(dir) == 0 {
		if _, ok := a.ensureThumbnails(r, id); !ok {
			writeErr(w, http.StatusNotFound, "thumbnails unavailable for this video")
			return
		}
	}
	path := media.ThumbnailPath(dir, n)
	if path == "" {
		writeErr(w, http.StatusNotFound, "thumbnail not found")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, http.StatusNotFound, "thumbnail not found")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, path)
}
