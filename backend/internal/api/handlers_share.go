package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/media"
	"videocms/backend/internal/models"
)

const defaultShareHours = 24 * 7

// newShareToken returns an unguessable token for a public share link.
func newShareToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createShare makes a short-lived public share link for a video.
// POST /api/videos/{id}/share  body: {"hours": n}
func (a *App) createShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	hours := defaultShareHours
	if r.Body != nil {
		var body struct {
			Hours int `json:"hours"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err == nil && body.Hours > 0 {
			hours = body.Hours
		}
	}
	if hours > 24*365 {
		hours = 24 * 365
	}

	var visible bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM videos v WHERE v.id=$1 AND `+visibleEpisodes(2)+`)`,
		id, user.ID).Scan(&visible); err != nil || !visible {
		writeErr(w, http.StatusNotFound, "video not found or not visible")
		return
	}

	token, err := newShareToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot generate share token")
		return
	}
	expires := time.Now().Add(time.Duration(hours) * time.Hour)
	if _, err := a.pool.Exec(r.Context(),
		`INSERT INTO share_tokens (video_id, token, expires_at, created_by) VALUES ($1,$2,$3,$4)`,
		id, token, expires, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "create share failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":            token,
		"url":              "/share/" + token,
		"expires_at":       expires,
		"expires_in_hours": hours,
	})
}

// listShares returns the share links for a video that the current user may
// manage (own links, or all links for admins).
// GET /api/videos/{id}/shares
func (a *App) listShares(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT st.token, st.expires_at, st.created_at, COALESCE(u.username, '')
		FROM share_tokens st
		LEFT JOIN users u ON u.id = st.created_by
		WHERE st.video_id = $1 AND (st.created_by = $2 OR $3::boolean)
		ORDER BY st.created_at DESC`,
		id, user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list shares failed")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var token, username string
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&token, &expiresAt, &createdAt, &username); err != nil {
			writeErr(w, http.StatusInternalServerError, "list shares failed")
			return
		}
		items = append(items, map[string]any{
			"token":      token,
			"url":        "/share/" + token,
			"expires_at": expiresAt,
			"created_at": createdAt,
			"created_by": username,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// deleteShare revokes a share link. The creator (or an admin) may revoke.
// DELETE /api/share/{token}
func (a *App) deleteShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	token := r.PathValue("token")
	tag, err := a.pool.Exec(r.Context(),
		`DELETE FROM share_tokens WHERE token=$1 AND (created_by=$2 OR $3::boolean)`,
		token, user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "revoke share failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "share link not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "share link revoked"})
}

// shareVideo resolves a public share token to the video, enforcing expiry,
// availability, library blocking and admin title blocking. It is the only
// entry point for the unauthenticated share routes.
func (a *App) shareVideo(r *http.Request, token string) (models.Video, string, time.Time, bool) {
	var v models.Video
	var posterPath, subtitlePath, filePath, container string
	var expiresAt time.Time
	err := a.pool.QueryRow(r.Context(), `
		SELECT v.id, v.title, v.filename, v.year, v.synopsis, v.genres,
		       v.duration_sec, v.width, v.height, v.video_codec, v.size_bytes,
		       v.container, v.file_path, v.poster_path, v.subtitle_path,
		       st.expires_at
		FROM share_tokens st
		JOIN videos v ON v.id = st.video_id
		WHERE st.token = $1 AND st.expires_at > now() AND v.available
		  AND NOT EXISTS (SELECT 1 FROM libraries lb WHERE lb.id = v.library_id AND lb.blocked)
		  AND NOT EXISTS (SELECT 1 FROM blocked_titles bt
		                  WHERE position(lower(bt.title) in lower(v.title)) > 0)`,
		token).Scan(
		&v.ID, &v.Title, &v.Filename, &v.Year, &v.Synopsis, &v.Genres,
		&v.DurationSec, &v.Width, &v.Height, &v.VideoCodec, &v.SizeBytes,
		&container, &filePath, &posterPath, &subtitlePath, &expiresAt)
	if err != nil {
		return models.Video{}, "", time.Time{}, false
	}
	v.Container = container
	v.Available = true
	v.HasPoster = posterPath != ""
	v.HasSubtitle = subtitlePath != ""
	v.PosterPath = posterPath
	v.SubtitlePath = subtitlePath
	return v, filePath, expiresAt, true
}

// GET /api/share/{token}/info
func (a *App) shareInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	v, _, expiresAt, ok := a.shareVideo(r, token)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"video":      v,
		"url":        "/share/" + token,
		"expires_at": expiresAt,
	})
}

// GET /api/share/{token}/stream
func (a *App) shareStream(w http.ResponseWriter, r *http.Request) {
	_, path, _, ok := a.shareVideo(r, r.PathValue("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), "inline")
}

// GET /api/share/{token}/download
func (a *App) shareDownload(w http.ResponseWriter, r *http.Request) {
	v, path, _, ok := a.shareVideo(r, r.PathValue("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	disposition := "attachment; filename=\"" + mime.QEncoding.Encode("UTF-8", v.Filename) + "\""
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), disposition)
}

// GET /api/share/{token}/poster
func (a *App) sharePoster(w http.ResponseWriter, r *http.Request) {
	v, _, _, ok := a.shareVideo(r, r.PathValue("token"))
	if !ok || !v.HasPoster {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, v.PosterPath)
}

// GET /api/share/{token}/subtitles
func (a *App) shareSubtitles(w http.ResponseWriter, r *http.Request) {
	v, _, _, ok := a.shareVideo(r, r.PathValue("token"))
	if !ok || !v.HasSubtitle {
		writeErr(w, http.StatusNotFound, "subtitles not found")
		return
	}
	serveSubtitleFile(w, r, v.SubtitlePath)
}

// GET /api/share/{token}/hls/{file...}
func (a *App) shareHLS(w http.ResponseWriter, r *http.Request) {
	v, filePath, _, ok := a.shareVideo(r, r.PathValue("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	a.serveHLS(w, r, v.ID, hlsVideo{
		FilePath:     filePath,
		Width:        v.Width,
		Height:       v.Height,
		SubtitlePath: v.SubtitlePath,
		Available:    true,
	}, r.PathValue("file"))
}
