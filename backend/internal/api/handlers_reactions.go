package api

import (
	"net/http"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

// PUT /api/videos/{id}/reaction — value 1 (like), -1 (dislike), 0 (clear).
func (a *App) setVideoReaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req struct {
		Value int `json:"value"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := auth.UserFrom(r)
	if req.Value == 0 {
		if _, err := a.pool.Exec(r.Context(),
			`DELETE FROM video_reactions WHERE video_id=$1 AND user_id=$2`, id, user.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "clear reaction failed")
			return
		}
	} else if req.Value == 1 || req.Value == -1 {
		if _, err := a.pool.Exec(r.Context(), `
			INSERT INTO video_reactions (video_id, user_id, value, updated_at)
			VALUES ($1,$2,$3,now())
			ON CONFLICT (video_id, user_id) DO UPDATE SET
				value=EXCLUDED.value, updated_at=now()`,
			id, user.ID, req.Value); err != nil {
			writeErr(w, http.StatusInternalServerError, "save reaction failed")
			return
		}
	} else {
		writeErr(w, http.StatusBadRequest, "value must be 1, -1 or 0")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "saved"})
}

// GET /api/videos/{id}/reactions
func (a *App) videoReactions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var likes, dislikes int
	if err := a.pool.QueryRow(r.Context(), `
		SELECT count(*) FILTER (WHERE value=1), count(*) FILTER (WHERE value=-1)
		FROM video_reactions WHERE video_id=$1`, id).Scan(&likes, &dislikes); err != nil {
		writeErr(w, http.StatusInternalServerError, "query reactions failed")
		return
	}
	mine := 0
	_ = a.pool.QueryRow(r.Context(),
		`SELECT value FROM video_reactions WHERE video_id=$1 AND user_id=$2`,
		id, auth.UserFrom(r).ID).Scan(&mine)
	writeJSON(w, http.StatusOK, map[string]any{
		"likes": likes, "dislikes": dislikes, "mine": mine,
	})
}
