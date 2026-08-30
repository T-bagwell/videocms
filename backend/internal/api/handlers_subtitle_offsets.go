package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
)

// maxSubtitleOffsetMs caps the subtitle sync adjustment at ±5 minutes.
const maxSubtitleOffsetMs int64 = 300000

type setSubtitleOffsetRequest struct {
	VideoID  uuid.UUID `json:"video_id"`
	OffsetMs int64     `json:"offset_ms"`
}

func clampSubtitleOffset(ms int64) int64 {
	if ms > maxSubtitleOffsetMs {
		return maxSubtitleOffsetMs
	}
	if ms < -maxSubtitleOffsetMs {
		return -maxSubtitleOffsetMs
	}
	return ms
}

// GET /api/users/me/subtitle-offset?video_id=… — the current user's subtitle
// sync offset for a video (0 when unset).
func (a *App) getUserSubtitleOffset(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	videoID, err := uuid.Parse(r.URL.Query().Get("video_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "video_id is required")
		return
	}
	var offset int64
	err = a.pool.QueryRow(r.Context(),
		`SELECT offset_ms FROM subtitle_offsets WHERE user_id=$1 AND video_id=$2`,
		user.ID, videoID).Scan(&offset)
	if errors.Is(err, pgx.ErrNoRows) {
		offset = 0
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "load subtitle offset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offset_ms": offset})
}

// PUT /api/users/me/subtitle-offset — save the user's subtitle sync offset.
func (a *App) setUserSubtitleOffset(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req setSubtitleOffsetRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT true FROM videos WHERE id=$1`, req.VideoID).Scan(&exists); err != nil {
		writeErr(w, http.StatusBadRequest, "video not found")
		return
	}
	offset := clampSubtitleOffset(req.OffsetMs)
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO subtitle_offsets (user_id, video_id, offset_ms)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, video_id)
		DO UPDATE SET offset_ms=EXCLUDED.offset_ms, updated_at=now()`,
		user.ID, req.VideoID, offset); err != nil {
		writeErr(w, http.StatusInternalServerError, "save subtitle offset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offset_ms": offset})
}

// DELETE /api/users/me/subtitle-offset?video_id=… — reset the offset to 0.
func (a *App) clearUserSubtitleOffset(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	videoID, err := uuid.Parse(r.URL.Query().Get("video_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "video_id is required")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM subtitle_offsets WHERE user_id=$1 AND video_id=$2`,
		user.ID, videoID); err != nil {
		writeErr(w, http.StatusInternalServerError, "clear subtitle offset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offset_ms": 0})
}
