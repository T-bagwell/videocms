package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

type transcriptStatus struct {
	Status  string `json:"status"`
	Lang    string `json:"lang"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
	Preview string `json:"preview,omitempty"`
}

func (a *App) loadTranscript(videoID uuid.UUID) transcriptStatus {
	var t transcriptStatus
	err := a.pool.QueryRow(context.Background(), `
		SELECT status, lang, path, error, text FROM video_transcripts WHERE video_id=$1`,
		videoID).Scan(&t.Status, &t.Lang, &t.Path, &t.Error, &t.Preview)
	if err != nil {
		return transcriptStatus{Status: "none"}
	}
	if len(t.Preview) > 500 {
		t.Preview = t.Preview[:500] + "…"
	}
	return t
}

// GET /api/videos/{id}/transcripts — transcription status.
func (a *App) getTranscript(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	writeJSON(w, http.StatusOK, a.loadTranscript(id))
}

// POST /api/videos/{id}/transcribe — run local Whisper on the video (admin).
func (a *App) transcribeVideo(w http.ResponseWriter, r *http.Request) {
	if a.cfg.WhisperBin == "" {
		writeErr(w, http.StatusBadRequest, "whisper is not configured (set WHISPER_BIN and WHISPER_MODEL)")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM videos WHERE id=$1 AND available`, id).Scan(&path); err != nil {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	cur := a.loadTranscript(id)
	if cur.Status == "done" || cur.Status == "pending" {
		writeJSON(w, http.StatusOK, cur)
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO video_transcripts (video_id, status)
		VALUES ($1, 'pending')
		ON CONFLICT (video_id) DO UPDATE SET status='pending', error='', updated_at=now()`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "start transcription failed")
		return
	}

	dir := filepath.Join(a.cfg.DataDir, "transcripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "create transcript directory failed")
		return
	}
	vttPath := filepath.Join(dir, id.String()+".vtt")
	if err := media.Transcribe(r.Context(), a.cfg.WhisperBin, a.cfg.WhisperModel, path, vttPath); err != nil {
		_, _ = a.pool.Exec(r.Context(), `
			UPDATE video_transcripts SET status='failed', error=$2, updated_at=now() WHERE video_id=$1`,
			id, err.Error())
		log.Printf("transcribe %s: %v", id.String()[:8], err)
		writeErr(w, http.StatusInternalServerError, "transcription failed: "+err.Error())
		return
	}
	data, err := os.ReadFile(vttPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read transcript failed")
		return
	}
	text := media.VTTText(data)
	if _, err := a.pool.Exec(r.Context(), `
		UPDATE video_transcripts SET status='done', path=$2, text=$3, error='', updated_at=now()
		WHERE video_id=$1`, id, vttPath, text); err != nil {
		writeErr(w, http.StatusInternalServerError, "save transcript failed")
		return
	}
	// Expose the transcript as a selectable subtitle track.
	_, _ = a.pool.Exec(r.Context(), `
		INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
		SELECT $1, COALESCE(MAX(position), -1) + 1, '', 'Transcript', $2, 'upload', $3, -1
		FROM subtitle_tracks WHERE video_id=$1
		ON CONFLICT (video_id, source_key) WHERE source_key <> ''
		DO UPDATE SET path=EXCLUDED.path`,
		id, vttPath, "transcript:"+id.String())
	writeJSON(w, http.StatusOK, a.loadTranscript(id))
}
