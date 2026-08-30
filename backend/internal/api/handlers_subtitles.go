package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/media"
)

var uploadSubtitleExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true}

type subtitleTrackRow struct {
	ID          uuid.UUID
	Position    int
	Lang        string
	Title       string
	Path        string
	Kind        string
	SourceKey   string
	StreamIndex int
}

func (a *App) subtitleDir() string {
	return filepath.Join(a.cfg.DataDir, "subtitles")
}

func (a *App) loadSubtitleTrack(ctx context.Context, videoID, trackID uuid.UUID) (subtitleTrackRow, bool) {
	var t subtitleTrackRow
	err := a.pool.QueryRow(ctx, `
		SELECT id, position, lang, title, path, kind, source_key, stream_index
		FROM subtitle_tracks WHERE id=$1 AND video_id=$2`, trackID, videoID).
		Scan(&t.ID, &t.Position, &t.Lang, &t.Title, &t.Path, &t.Kind, &t.SourceKey, &t.StreamIndex)
	if err != nil {
		return subtitleTrackRow{}, false
	}
	return t, true
}

// ensureSubtitlePath returns a readable subtitle file for a track, extracting
// embedded tracks on first use.
func (a *App) ensureSubtitlePath(ctx context.Context, videoID uuid.UUID, t *subtitleTrackRow) (string, error) {
	if t.Path != "" {
		if _, err := os.Stat(t.Path); err == nil {
			return t.Path, nil
		}
	}
	if t.Kind != "embedded" || t.StreamIndex < 0 {
		return "", errors.New("subtitle file unavailable")
	}
	var input string
	if err := a.pool.QueryRow(ctx,
		`SELECT file_path FROM videos WHERE id=$1`, videoID).Scan(&input); err != nil {
		return "", err
	}
	dir := a.subtitleDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fmt.Sprintf("%s-e%d.vtt", videoID.String(), t.StreamIndex))
	if err := media.ExtractEmbeddedSubtitle(ctx, a.ffmpegBin(), input, t.StreamIndex, dst); err != nil {
		return "", err
	}
	t.Path = dst
	if _, err := a.pool.Exec(ctx,
		`UPDATE subtitle_tracks SET path=$1 WHERE id=$2`, dst, t.ID); err != nil {
		return "", err
	}
	return dst, nil
}

// listSubtitleTracks returns all subtitle tracks of a video with the active
// flag, so the player can offer language switching.
// GET /api/videos/{id}/subtitle-tracks
func (a *App) listSubtitleTracks(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	user := auth.UserFrom(r)
	var active string
	_ = a.pool.QueryRow(r.Context(),
		`SELECT subtitle_path FROM videos WHERE id=$1`, id).Scan(&active)

	items, err := a.listTracksForVideo(r.Context(), id, active, &user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list subtitle tracks failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// getSubtitleTrack serves the WebVTT for one specific track, extracting
// embedded tracks on first request.
// GET /api/videos/{id}/subtitles/{trackId}
func (a *App) getSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid track id")
		return
	}
	t, ok := a.loadSubtitleTrack(r.Context(), id, trackID)
	if !ok {
		writeErr(w, http.StatusNotFound, "subtitle track not found")
		return
	}
	path, err := a.ensureSubtitlePath(r.Context(), id, &t)
	if err != nil {
		log.Printf("serve subtitle track %s: %v", trackID.String()[:8], err)
		writeErr(w, http.StatusInternalServerError, "subtitle unavailable: "+err.Error())
		return
	}
	offsetMs := int64(0)
	if q := r.URL.Query().Get("offset_ms"); q != "" {
		if n, err := strconv.ParseInt(q, 10, 64); err == nil {
			offsetMs = clampSubtitleOffset(n)
		}
	}
	serveSubtitleFileOffset(w, r, path, offsetMs)
}

// setActiveSubtitleTrack saves the current user's subtitle preference for a
// video.
// PUT /api/videos/{id}/subtitles/{trackId}/active
func (a *App) setActiveSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid track id")
		return
	}
	_, ok := a.loadSubtitleTrack(r.Context(), id, trackID)
	if !ok {
		writeErr(w, http.StatusNotFound, "subtitle track not found")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO user_subtitle_prefs (user_id, video_id, track_id)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id, video_id)
		DO UPDATE SET track_id=EXCLUDED.track_id, updated_at=now()`,
		user.ID, id, trackID); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle preference saved"})
}

// clearSubtitlePreference removes the current user's personal subtitle choice
// so the global default applies again.
// DELETE /api/videos/{id}/subtitles/preference
func (a *App) clearSubtitlePreference(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM user_subtitle_prefs WHERE user_id=$1 AND video_id=$2`,
		user.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "clear preference failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle preference cleared"})
}

// setGlobalSubtitleDefault makes a track the video's default subtitle for
// everyone (admin only). Personal preferences still win per user.
// PUT /api/videos/{id}/subtitles/{trackId}/default
func (a *App) setGlobalSubtitleDefault(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid track id")
		return
	}
	t, ok := a.loadSubtitleTrack(r.Context(), id, trackID)
	if !ok {
		writeErr(w, http.StatusNotFound, "subtitle track not found")
		return
	}
	path, err := a.ensureSubtitlePath(r.Context(), id, &t)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "subtitle unavailable: "+err.Error())
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, path, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "global subtitle default set"})
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
	defer func() { _ = file.Close() }()

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
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save subtitle")
		return
	}

	var pos int
	if err := a.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(position), -1) + 1 FROM subtitle_tracks WHERE video_id=$1`, id).Scan(&pos); err != nil {
		writeErr(w, http.StatusInternalServerError, "create track failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
		VALUES ($1,$2,'',$3,$4,'upload','upload:'||$3,-1)`,
		id, pos, header.Filename, dst); err != nil {
		writeErr(w, http.StatusInternalServerError, "create track failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, dst, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle uploaded"})
}

// deleteSubtitle removes the active subtitle (track + file) of a video and
// promotes the next track if one remains.
// DELETE /api/videos/{id}/subtitles
func (a *App) deleteSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var active string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT subtitle_path FROM videos WHERE id=$1`, id).Scan(&active); err != nil || active == "" {
		writeErr(w, http.StatusNotFound, "no subtitle to remove")
		return
	}

	var trackID uuid.UUID
	var trackPath, trackKind string
	err = a.pool.QueryRow(r.Context(), `
		SELECT id, path, kind FROM subtitle_tracks
		WHERE video_id=$1 AND path=$2 ORDER BY position LIMIT 1`, id, active).
		Scan(&trackID, &trackPath, &trackKind)
	if err == nil {
		if _, err := a.pool.Exec(r.Context(),
			`DELETE FROM subtitle_tracks WHERE id=$1`, trackID); err != nil {
			writeErr(w, http.StatusInternalServerError, "remove track failed")
			return
		}
		removeOldSubtitle(a.subtitleDir(), trackPath, "")
	}

	// promote the first remaining track, if any
	next := subtitleTrackRow{}
	err = a.pool.QueryRow(r.Context(), `
		SELECT id, position, lang, title, path, kind, source_key, stream_index
		FROM subtitle_tracks WHERE video_id=$1 ORDER BY position LIMIT 1`, id).
		Scan(&next.ID, &next.Position, &next.Lang, &next.Title, &next.Path, &next.Kind, &next.SourceKey, &next.StreamIndex)
	if err == nil {
		p, err := a.ensureSubtitlePath(r.Context(), id, &next)
		if err == nil {
			active = p
		}
	} else {
		active = ""
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, active, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subtitle removed"})
}

// extractEmbeddedSubtitle makes sure every text subtitle stream inside the
// video container has a track row and extracts the ones that are not yet on
// disk. Image-based tracks (PGS, VobSub…) are skipped.
// POST /api/videos/{id}/subtitles/extract
func (a *App) extractEmbeddedSubtitle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var input string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM videos WHERE id=$1`, id).Scan(&input); err != nil {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}

	streams, err := media.ProbeSubtitleStreams(r.Context(), a.ffprobeBin(), input)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	var text []media.SubtitleStream
	for _, s := range streams {
		if !media.IsImageSubtitleCodec(s.Codec) {
			text = append(text, s)
		}
	}
	if len(text) == 0 {
		if len(streams) == 0 {
			writeErr(w, http.StatusBadRequest, "no embedded subtitle streams found")
			return
		}
		writeErr(w, http.StatusBadRequest, "embedded subtitles are image-based and cannot be extracted")
		return
	}

	// ensure a track row exists for every text stream
	var basePos int
	_ = a.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(position), -1) + 1 FROM subtitle_tracks WHERE video_id=$1 AND kind='embedded'`, id).
		Scan(&basePos)
	for i, s := range text {
		if _, err := a.pool.Exec(r.Context(), `
			INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
			VALUES ($1,$2,$3,$4,'','embedded','embedded:'||$5,$5)
			ON CONFLICT (video_id, source_key) WHERE source_key <> '' DO NOTHING`,
			id, basePos+i, s.Language, s.Title, s.Index); err != nil {
			writeErr(w, http.StatusInternalServerError, "create track failed")
			return
		}
	}

	rows, err := a.pool.Query(r.Context(), `
		SELECT id, stream_index FROM subtitle_tracks
		WHERE video_id=$1 AND kind='embedded' AND path=''`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load tracks failed")
		return
	}
	defer rows.Close()
	extracted := 0
	for rows.Next() {
		var trackID uuid.UUID
		var streamIndex int
		if err := rows.Scan(&trackID, &streamIndex); err != nil {
			continue
		}
		t := subtitleTrackRow{ID: trackID, Kind: "embedded", StreamIndex: streamIndex}
		if _, err := a.ensureSubtitlePath(r.Context(), id, &t); err == nil {
			extracted++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": fmt.Sprintf("%d embedded subtitle track(s) extracted", extracted),
		"count":   extracted,
	})
}

// removeOldSubtitle deletes a previously generated/uploaded subtitle file, but
// never a sidecar file next to the video (those belong to the media folder).
func removeOldSubtitle(dir, oldPath, keep string) {
	if oldPath == "" || oldPath == keep {
		return
	}
	prefix := filepath.Clean(dir) + string(os.PathSeparator)
	if strings.HasPrefix(filepath.Clean(oldPath), prefix) {
		_ = os.Remove(oldPath)
	}
}

func (a *App) ffprobeBin() string {
	return media.ResolveTool("ffprobe")
}

func (a *App) ffmpegBin() string {
	return media.ResolveTool("ffmpeg")
}
