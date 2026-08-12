package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

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

type shareRequest struct {
	Hours    int    `json:"hours"`
	Password string `json:"password"`
}

// parseShareRequest reads hours and an optional password from the request
// body, returning the expiry hours and the bcrypt hash of the password ("" when
// no password was set).
func parseShareRequest(r *http.Request) (int, string, error) {
	hours := defaultShareHours
	password := ""
	if r.Body != nil {
		var body shareRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil && err != io.EOF {
			return hours, "", err
		}
		if body.Hours > 0 {
			hours = body.Hours
		}
		password = body.Password
	}
	if hours > 24*365 {
		hours = 24 * 365
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return hours, "", err
		}
		password = string(hash)
	}
	return hours, password, nil
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func (a *App) insertShare(ctx context.Context, scope string, videoID, seriesID, playlistID uuid.UUID, expires time.Time, passwordHash string, userID uuid.UUID) (string, error) {
	token, err := newShareToken()
	if err != nil {
		return "", err
	}
	_, err = a.pool.Exec(ctx, `
		INSERT INTO share_tokens (scope, video_id, series_id, playlist_id, token, expires_at, password_hash, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		scope, nullableUUID(videoID), nullableUUID(seriesID), nullableUUID(playlistID), token, expires, passwordHash, userID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func writeShareCreated(w http.ResponseWriter, token string, expires time.Time, hours int) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":            token,
		"url":              "/share/" + token,
		"expires_at":       expires,
		"expires_in_hours": hours,
	})
}

// createVideoShare makes a short-lived public share link for a video.
// POST /api/videos/{id}/share  body: {"hours": n}
func (a *App) createVideoShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var visible bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM videos v WHERE v.id=$1 AND `+visibleEpisodes(2)+`)`,
		id, user.ID).Scan(&visible); err != nil || !visible {
		writeErr(w, http.StatusNotFound, "video not found or not visible")
		return
	}
	hours, passwordHash, err := parseShareRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid share request")
		return
	}
	expires := time.Now().Add(time.Duration(hours) * time.Hour)
	token, err := a.insertShare(r.Context(), "video", id, uuid.Nil, uuid.Nil, expires, passwordHash, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create share failed")
		return
	}
	writeShareCreated(w, token, expires, hours)
}

// createSeriesShare makes a share link for an entire TV show.
// POST /api/series/{id}/share  body: {"hours": n}
func (a *App) createSeriesShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	var visible bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM series s JOIN videos v ON v.series_id=s.id
		               WHERE s.id=$1 AND `+visibleEpisodes(2)+`)`,
		id, user.ID).Scan(&visible); err != nil || !visible {
		writeErr(w, http.StatusNotFound, "series not found or not visible")
		return
	}
	hours, passwordHash, err := parseShareRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid share request")
		return
	}
	expires := time.Now().Add(time.Duration(hours) * time.Hour)
	token, err := a.insertShare(r.Context(), "series", uuid.Nil, id, uuid.Nil, expires, passwordHash, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create share failed")
		return
	}
	writeShareCreated(w, token, expires, hours)
}

// createPlaylistShare makes a share link for a playlist owned by the user.
// POST /api/playlists/{id}/share  body: {"hours": n}
func (a *App) createPlaylistShare(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	hours, passwordHash, err := parseShareRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid share request")
		return
	}
	expires := time.Now().Add(time.Duration(hours) * time.Hour)
	token, err := newShareToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot generate share token")
		return
	}
	tag, err := a.pool.Exec(r.Context(), `
		INSERT INTO share_tokens (scope, playlist_id, token, expires_at, password_hash, created_by)
		SELECT 'playlist', id, $1, $2, $3, $4 FROM playlists WHERE id=$5 AND user_id=$4`,
		token, expires, passwordHash, user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create share failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "playlist not found")
		return
	}
	writeShareCreated(w, token, expires, hours)
}

// listSharesFor returns the share links for a video/series/playlist that the
// current user may manage (own links, or all links for admins).
func (a *App) listSharesFor(w http.ResponseWriter, r *http.Request, col string, id uuid.UUID) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT st.token, st.expires_at, st.created_at, COALESCE(u.username, ''),
		       st.password_hash <> ''
		FROM share_tokens st
		LEFT JOIN users u ON u.id = st.created_by
		WHERE st.%s = $1 AND (st.created_by = $2 OR $3::boolean)
		ORDER BY st.created_at DESC`, col),
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
		var passwordProtected bool
		if err := rows.Scan(&token, &expiresAt, &createdAt, &username, &passwordProtected); err != nil {
			writeErr(w, http.StatusInternalServerError, "list shares failed")
			return
		}
		items = append(items, map[string]any{
			"token":              token,
			"url":                "/share/" + token,
			"expires_at":         expiresAt,
			"created_at":         createdAt,
			"created_by":         username,
			"password_protected": passwordProtected,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) listVideoShares(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	a.listSharesFor(w, r, "video_id", id)
}

func (a *App) listSeriesShares(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	a.listSharesFor(w, r, "series_id", id)
}

func (a *App) listPlaylistShares(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	a.listSharesFor(w, r, "playlist_id", id)
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

type shareTokenRow struct {
	Scope        string
	ExpiresAt    time.Time
	VideoID      uuid.UUID
	SeriesID     uuid.UUID
	PlaylistID   uuid.UUID
	PasswordHash string
}

func (a *App) resolveShareToken(r *http.Request, token string) (shareTokenRow, bool) {
	var row shareTokenRow
	err := a.pool.QueryRow(r.Context(), `
		SELECT scope, expires_at, password_hash,
		       COALESCE(video_id, '00000000-0000-0000-0000-000000000000'),
		       COALESCE(series_id, '00000000-0000-0000-0000-000000000000'),
		       COALESCE(playlist_id, '00000000-0000-0000-0000-000000000000')
		FROM share_tokens WHERE token=$1 AND expires_at > now()`, token).
		Scan(&row.Scope, &row.ExpiresAt, &row.PasswordHash, &row.VideoID, &row.SeriesID, &row.PlaylistID)
	if err != nil {
		return shareTokenRow{}, false
	}
	return row, true
}

// sharePasswordOK verifies an optional share password from the
// X-Share-Password header or the ?pw= query parameter.
func sharePasswordOK(r *http.Request, hash string) bool {
	if hash == "" {
		return true
	}
	pw := r.Header.Get("X-Share-Password")
	if pw == "" {
		pw = r.URL.Query().Get("pw")
	}
	if pw == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// shareGuard resolves the token and enforces expiry and the optional password,
// writing the error response itself when access is denied.
func (a *App) shareGuard(w http.ResponseWriter, r *http.Request) (shareTokenRow, bool) {
	row, ok := a.resolveShareToken(r, r.PathValue("token"))
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return shareTokenRow{}, false
	}
	if !sharePasswordOK(r, row.PasswordHash) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":             "share link requires a password",
			"password_required": true,
		})
		return shareTokenRow{}, false
	}
	return row, true
}

// publicVideoCond matches videos that are available, not in a blocked library
// and not under an admin title block — the public visibility contract for
// share links (hidden-path filters are per-user and do not apply).
func publicVideoCond() string {
	return `v.available
		AND NOT EXISTS (SELECT 1 FROM libraries lb WHERE lb.id = v.library_id AND lb.blocked)
		AND NOT EXISTS (SELECT 1 FROM blocked_titles bt
		                WHERE position(lower(bt.title) in lower(v.title)) > 0)`
}

// shareVideoForToken resolves a public share token + video id pair. The video
// must belong to the shared scope (video/series/playlist) and pass the public
// visibility contract.
func (a *App) shareVideoForToken(r *http.Request, token string, videoID uuid.UUID) (models.Video, string, bool) {
	var v models.Video
	var posterPath, subtitlePath, filePath, container string
	err := a.pool.QueryRow(r.Context(), `
		SELECT v.id, v.title, v.filename, v.year, v.synopsis, v.genres,
		       v.duration_sec, v.width, v.height, v.video_codec, v.size_bytes,
		       v.container, v.file_path, v.poster_path, v.subtitle_path
		FROM share_tokens st
		JOIN videos v ON v.id = $2
		WHERE st.token = $1 AND st.expires_at > now() AND `+publicVideoCond()+`
		  AND (st.scope = 'video' AND st.video_id = v.id
		       OR st.scope = 'series' AND v.series_id = st.series_id
		       OR st.scope = 'playlist' AND EXISTS (
		           SELECT 1 FROM playlist_items pi
		           WHERE pi.playlist_id = st.playlist_id AND pi.video_id = v.id))`,
		token, videoID).Scan(
		&v.ID, &v.Title, &v.Filename, &v.Year, &v.Synopsis, &v.Genres,
		&v.DurationSec, &v.Width, &v.Height, &v.VideoCodec, &v.SizeBytes,
		&container, &filePath, &posterPath, &subtitlePath)
	if err != nil {
		return models.Video{}, "", false
	}
	v.Container = container
	v.Available = true
	v.HasPoster = posterPath != ""
	v.HasSubtitle = subtitlePath != ""
	v.PosterPath = posterPath
	v.SubtitlePath = subtitlePath
	return v, filePath, true
}

type publicVideo struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Filename    string    `json:"filename"`
	Year        int       `json:"year"`
	DurationSec float64   `json:"duration_sec"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	VideoCodec  string    `json:"video_codec"`
	SizeBytes   int64     `json:"size_bytes"`
	Season      int       `json:"season,omitempty"`
	Episode     int       `json:"episode,omitempty"`
	HasPoster   bool      `json:"has_poster"`
	HasSubtitle bool      `json:"has_subtitle"`
}

func (a *App) listPublicVideos(ctx context.Context, where string, args ...any) ([]publicVideo, error) {
	query := `
		SELECT v.id, v.title, v.filename, v.year, v.duration_sec, v.width, v.height,
		       v.video_codec, v.size_bytes, v.season, v.episode,
		       v.poster_path, v.subtitle_path
		FROM videos v
		WHERE ` + where
	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []publicVideo{}
	for rows.Next() {
		var p publicVideo
		var posterPath, subtitlePath string
		if err := rows.Scan(&p.ID, &p.Title, &p.Filename, &p.Year, &p.DurationSec,
			&p.Width, &p.Height, &p.VideoCodec, &p.SizeBytes, &p.Season, &p.Episode,
			&posterPath, &subtitlePath); err != nil {
			return nil, err
		}
		p.HasPoster = posterPath != ""
		p.HasSubtitle = subtitlePath != ""
		items = append(items, p)
	}
	return items, rows.Err()
}

// GET /api/share/{token}/info
func (a *App) shareInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	row, ok := a.shareGuard(w, r)
	if !ok {
		return
	}
	base := map[string]any{
		"scope":      row.Scope,
		"url":        "/share/" + token,
		"expires_at": row.ExpiresAt,
	}

	switch row.Scope {
	case "series":
		var name string
		var season, episodeCount int
		err := a.pool.QueryRow(r.Context(), fmt.Sprintf(`
			SELECT s.name, s.season,
			       (SELECT count(*) FROM videos v WHERE v.series_id=s.id AND %s) AS episode_count
			FROM series s WHERE s.id=$1`, publicVideoCond()), row.SeriesID).
			Scan(&name, &season, &episodeCount)
		if err != nil {
			writeErr(w, http.StatusNotFound, "series not found")
			return
		}
		items, err := a.listPublicVideos(r.Context(),
			`v.series_id=$1 AND `+publicVideoCond()+` ORDER BY v.season, v.episode`, row.SeriesID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "load series items failed")
			return
		}
		base["series"] = map[string]any{
			"id":            row.SeriesID,
			"name":          name,
			"season":        season,
			"episode_count": episodeCount,
		}
		base["items"] = items

	case "playlist":
		var name string
		if err := a.pool.QueryRow(r.Context(),
			`SELECT name FROM playlists WHERE id=$1`, row.PlaylistID).Scan(&name); err != nil {
			writeErr(w, http.StatusNotFound, "playlist not found")
			return
		}
		items, err := a.listPublicVideos(r.Context(), `
			EXISTS (SELECT 1 FROM playlist_items pi
			        WHERE pi.playlist_id=$1 AND pi.video_id=v.id)
			AND `+publicVideoCond()+` ORDER BY (
			    SELECT pi.position FROM playlist_items pi
			    WHERE pi.playlist_id=$1 AND pi.video_id=v.id)`, row.PlaylistID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "load playlist items failed")
			return
		}
		base["playlist"] = map[string]any{"id": row.PlaylistID, "name": name}
		base["items"] = items

	default:
		v, _, ok := a.shareVideoForToken(r, token, row.VideoID)
		if !ok {
			writeErr(w, http.StatusNotFound, "video not found")
			return
		}
		tracks, err := a.listTracksForVideo(r.Context(), v.ID, v.SubtitlePath, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "load subtitle tracks failed")
			return
		}
		base["video"] = v
		base["subtitle_tracks"] = tracks
	}
	writeJSON(w, http.StatusOK, base)
}

func (a *App) listTracksForVideo(ctx context.Context, videoID uuid.UUID, activePath string, userID *uuid.UUID) ([]models.SubtitleTrack, error) {
	var pref uuid.UUID
	hasPref := false
	if userID != nil {
		err := a.pool.QueryRow(ctx,
			`SELECT track_id FROM user_subtitle_prefs WHERE user_id=$1 AND video_id=$2`,
			*userID, videoID).Scan(&pref)
		hasPref = err == nil
	}
	rows, err := a.pool.Query(ctx, `
		SELECT id, position, lang, title, path, kind
		FROM subtitle_tracks WHERE video_id=$1 ORDER BY position`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.SubtitleTrack{}
	for rows.Next() {
		var t models.SubtitleTrack
		var path, kind string
		if err := rows.Scan(&t.ID, &t.Position, &t.Lang, &t.Title, &path, &kind); err != nil {
			return nil, err
		}
		t.Kind = kind
		if hasPref {
			t.IsActive = t.ID == pref
		} else {
			t.IsActive = path != "" && path == activePath
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// shareVideoFromRequest resolves the token + {videoId} path pair for the
// per-video public media endpoints, enforcing the share password first.
func (a *App) shareVideoFromRequest(w http.ResponseWriter, r *http.Request) (models.Video, string, bool) {
	if _, ok := a.shareGuard(w, r); !ok {
		return models.Video{}, "", false
	}
	videoID, err := uuid.Parse(r.PathValue("videoId"))
	if err != nil {
		return models.Video{}, "", false
	}
	return a.shareVideoForToken(r, r.PathValue("token"), videoID)
}

// GET /api/share/{token}/video/{videoId}/stream
func (a *App) shareStream(w http.ResponseWriter, r *http.Request) {
	_, path, ok := a.shareVideoFromRequest(w, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), "inline")
}

// GET /api/share/{token}/video/{videoId}/download
func (a *App) shareDownload(w http.ResponseWriter, r *http.Request) {
	v, path, ok := a.shareVideoFromRequest(w, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	disposition := "attachment; filename=\"" + mime.QEncoding.Encode("UTF-8", v.Filename) + "\""
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), disposition)
}

// GET /api/share/{token}/video/{videoId}/poster
func (a *App) sharePoster(w http.ResponseWriter, r *http.Request) {
	v, _, ok := a.shareVideoFromRequest(w, r)
	if !ok || !v.HasPoster {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, v.PosterPath)
}

// GET /api/share/{token}/video/{videoId}/subtitles
func (a *App) shareSubtitles(w http.ResponseWriter, r *http.Request) {
	v, _, ok := a.shareVideoFromRequest(w, r)
	if !ok || !v.HasSubtitle {
		writeErr(w, http.StatusNotFound, "subtitles not found")
		return
	}
	serveSubtitleFile(w, r, v.SubtitlePath)
}

// GET /api/share/{token}/video/{videoId}/subtitle-tracks
func (a *App) shareSubtitleTracks(w http.ResponseWriter, r *http.Request) {
	v, _, ok := a.shareVideoFromRequest(w, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	items, err := a.listTracksForVideo(r.Context(), v.ID, v.SubtitlePath, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load subtitle tracks failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/share/{token}/video/{videoId}/subtitles/{trackId}
func (a *App) shareSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	v, _, ok := a.shareVideoFromRequest(w, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid track id")
		return
	}
	t, ok := a.loadSubtitleTrack(r.Context(), v.ID, trackID)
	if !ok {
		writeErr(w, http.StatusNotFound, "subtitle track not found")
		return
	}
	path, err := a.ensureSubtitlePath(r.Context(), v.ID, &t)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "subtitle unavailable: "+err.Error())
		return
	}
	serveSubtitleFile(w, r, path)
}

// GET /api/share/{token}/video/{videoId}/hls/{file...}
func (a *App) shareHLS(w http.ResponseWriter, r *http.Request) {
	v, _, ok := a.shareVideoFromRequest(w, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "share link not found or expired")
		return
	}
	hv, ok := a.videoForHLS(r.Context(), v.ID)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	a.serveHLS(w, r, v.ID, hv, r.PathValue("file"))
}
