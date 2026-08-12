package api

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

var uploadSubtitleExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true}

func (a *App) subtitleDir() string {
	return filepath.Join(a.cfg.DataDir, "subtitles")
}

// uploadSubtitle stores an uploaded subtitle file for a video and activates it.
// POST /api/videos/{id}/subtitles (multipart field "subtitle")
func (a *App) uploadSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("subtitle")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing subtitle file")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !uploadSubtitleExts[ext] {
		writeErr(w, http.StatusBadRequest, "subtitle must be .srt, .vtt, .ass or .ssa")
		return
	}

	dir := a.subtitleDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot create subtitle dir")
		return
	}
	dst := filepath.Join(dir, id.String()+"-upload"+ext)
	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save subtitle")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save subtitle")
		return
	}

	var oldPath string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT subtitle_path FROM videos WHERE id=$1`, id).Scan(&oldPath); err != nil {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	removeOldSubtitle(dir, oldPath, dst)
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, dst, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle uploaded"})
}

// deleteSubtitle removes the active subtitle (file + record) of a video.
// DELETE /api/videos/{id}/subtitles
func (a *App) deleteSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT subtitle_path FROM videos WHERE id=$1`, id).Scan(&path); err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "no subtitle to remove")
		return
	}
	removeOldSubtitle(a.subtitleDir(), path, "")
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path='', updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle removed"})
}

// extractEmbeddedSubtitle finds the first text subtitle stream inside the video
// container, converts it to WebVTT and activates it. Image-based tracks (PGS,
// VobSub…) are rejected because they cannot be converted without OCR.
// POST /api/videos/{id}/subtitles/extract
func (a *App) extractEmbeddedSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var input, oldPath string
	var available bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path, subtitle_path, available FROM videos WHERE id=$1`, id,
	).Scan(&input, &oldPath, &available); err != nil || !available {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}

	streams, err := media.ProbeSubtitleStreams(r.Context(), a.ffprobeBin(), input)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	idx := media.FirstTextSubtitle(streams)
	if idx < 0 {
		if len(streams) == 0 {
			writeErr(w, http.StatusBadRequest, "no embedded subtitle streams found")
			return
		}
		writeErr(w, http.StatusBadRequest, "embedded subtitles are image-based and cannot be extracted")
		return
	}

	dir := a.subtitleDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot create subtitle dir")
		return
	}
	dst := filepath.Join(dir, id.String()+"-embedded.vtt")
	if err := media.ExtractEmbeddedSubtitle(r.Context(), a.ffmpegBin(), input, idx, dst); err != nil {
		log.Printf("extract subtitle %s: %v", id.String()[:8], err)
		writeErr(w, http.StatusInternalServerError, "subtitle extraction failed: "+err.Error())
		return
	}
	removeOldSubtitle(dir, oldPath, dst)
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, dst, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "embedded subtitle extracted"})
}

// removeOldSubtitle deletes a previous generated/uploaded subtitle file, but
// never a sidecar file next to the video (those belong to the media folder).
func removeOldSubtitle(dir, oldPath, keep string) {
	if oldPath == "" || oldPath == keep {
		return
	}
	prefix := filepath.Clean(dir) + string(os.PathSeparator)
	if strings.HasPrefix(filepath.Clean(oldPath), prefix) {
		os.Remove(oldPath)
	}
}

func (a *App) ffprobeBin() string {
	return media.ResolveTool("ffprobe")
}

func (a *App) ffmpegBin() string {
	return media.ResolveTool("ffmpeg")
}
