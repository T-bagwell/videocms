package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

type searchSubtitlesRequest struct {
	Language string `json:"language"`
}

type downloadSubtitleRequest struct {
	FileID string `json:"file_id"`
}

// subtitleProvider returns the configured online subtitle provider (injectable
// for tests).
func (a *App) subtitleProvider() media.SubtitleProvider {
	if a.subProvider != nil {
		return a.subProvider
	}
	return media.NewOpenSubtitlesProvider(a.cfg.SubtitleOSUser, a.cfg.SubtitleOSPassword, a.cfg.SubtitleOSAPIKey)
}

// POST /api/videos/{id}/subtitles/search — find subtitle candidates online.
func (a *App) searchSubtitles(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req searchSubtitlesRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var title string
	var year int
	if err := a.pool.QueryRow(r.Context(),
		`SELECT title, year FROM videos WHERE id=$1`, id).Scan(&title, &year); err != nil {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	query := title
	if year > 0 {
		query += " " + strconv.Itoa(year)
	}
	items, err := a.subtitleProvider().Search(r.Context(), query, strings.TrimSpace(req.Language))
	if err != nil {
		log.Printf("subtitle search for %s: %v", id.String()[:8], err)
		writeErr(w, http.StatusBadGateway, "subtitle search failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/videos/{id}/subtitles/download — fetch a candidate, store it under
// DATA_DIR/subtitles/<video-id>/ and register it as an upload subtitle track.
func (a *App) downloadSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req downloadSubtitleRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.FileID) == "" {
		writeErr(w, http.StatusBadRequest, "file_id is required")
		return
	}
	content, err := a.subtitleProvider().Download(r.Context(), strings.TrimSpace(req.FileID))
	if err != nil {
		log.Printf("subtitle download for %s: %v", id.String()[:8], err)
		writeErr(w, http.StatusBadGateway, "subtitle download failed: "+err.Error())
		return
	}

	dir := filepath.Join(a.cfg.DataDir, "subtitles", id.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "create subtitle directory failed")
		return
	}
	base := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			return r
		}
		return '_'
	}, req.FileID)
	if base == "" || base == "." {
		base = "subtitle"
	}
	name := base + detectSubtitleExt(content)
	if !filepath.IsLocal(name) {
		writeErr(w, http.StatusBadRequest, "invalid subtitle file name")
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "save subtitle failed")
		return
	}

	var position int
	_ = a.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(position), -1) + 1 FROM subtitle_tracks WHERE video_id=$1`, id).Scan(&position)
	sourceKey := "os:" + req.FileID
	var trackID uuid.UUID
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
		VALUES ($1, $2, '', 'OpenSubtitles', $3, 'upload', $4, -1)
		ON CONFLICT (video_id, source_key) WHERE source_key <> ''
		DO UPDATE SET path=EXCLUDED.path
		RETURNING id`,
		id, position, path, sourceKey).Scan(&trackID); err != nil {
		log.Printf("insert downloaded subtitle: %v", err)
		writeErr(w, http.StatusInternalServerError, "register subtitle failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      trackID,
		"title":   "OpenSubtitles",
		"path":    path,
		"kind":    "upload",
		"format":  strings.TrimPrefix(filepath.Ext(path), "."),
		"message": "subtitle downloaded",
	})
}

// detectSubtitleExt guesses the container format from file content.
func detectSubtitleExt(content []byte) string {
	s := strings.TrimSpace(string(content))
	if strings.HasPrefix(s, "WEBVTT") {
		return ".vtt"
	}
	if strings.HasPrefix(s, "[Script Info]") || strings.Contains(s, "\nDialogue:") {
		return ".ass"
	}
	return ".srt"
}
