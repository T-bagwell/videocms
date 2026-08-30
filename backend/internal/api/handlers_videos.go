package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/media"
	"videocms/backend/internal/models"
)

type videoListResponse struct {
	Items    []models.Video `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

const videoColumns = `
	v.id, v.library_id, l.name, v.title, v.filename, v.file_path, v.size_bytes,
	v.duration_sec, v.width, v.height, v.video_codec, v.container, v.year,
	v.synopsis, v.genres, v.poster_path, v.subtitle_path, v.available,
	v.series_id, v.season, v.episode, COALESCE(s.name, ''),
	COALESCE(bl.id::text, '') AS blocked_id,
	v.created_at, v.updated_at,
	EXISTS(SELECT 1 FROM favorites f WHERE f.user_id=$1 AND f.video_id=v.id) AS is_fav,
	COALESCE((SELECT wp.position_sec FROM watch_progress wp WHERE wp.user_id=$1 AND wp.video_id=v.id), 0),
	COALESCE((SELECT wp.duration_sec FROM watch_progress wp WHERE wp.user_id=$1 AND wp.video_id=v.id), 0)`

// blockedLateral resolves the longest matching blocked-title rule for a video
// row (as alias bl); every query selecting videoColumns must join it.
const blockedLateral = `LEFT JOIN LATERAL (
	SELECT bt.id FROM blocked_titles bt
	WHERE position(lower(bt.title) in lower(v.title)) > 0
	ORDER BY length(bt.title) DESC
	LIMIT 1
) bl ON true`

// visiblePaths returns a SQL condition matching videos that are available and
// not located under any of the current user's hidden paths. userParam is the
// $N placeholder holding the user id.
func visiblePaths(userParam int) string {
	return fmt.Sprintf(`v.available AND NOT EXISTS (
		SELECT 1 FROM hidden_paths hp
		WHERE hp.user_id=$%d
		  AND (v.file_path = hp.path OR starts_with(v.file_path, hp.path || '/'))
	) AND NOT EXISTS (
		SELECT 1 FROM libraries lb WHERE lb.id = v.library_id AND lb.blocked
	)`, userParam)
}

// visibleEpisodes returns a SQL condition matching videos that are visible to
// the current user: available, not under a hidden path, and not blocked by an
// admin title filter. userParam is the $N placeholder holding the user id.
func visibleEpisodes(userParam int) string {
	return fmt.Sprintf(`%s AND %s`, visiblePaths(userParam), blockedTitlesCondition())
}

func scanVideo(row pgx.Row) (models.Video, error) {
	var v models.Video
	err := row.Scan(&v.ID, &v.LibraryID, &v.LibraryName, &v.Title, &v.Filename, &v.FilePath,
		&v.SizeBytes, &v.DurationSec, &v.Width, &v.Height, &v.VideoCodec, &v.Container,
		&v.Year, &v.Synopsis, &v.Genres, &v.PosterPath, &v.SubtitlePath, &v.Available,
		&v.SeriesID, &v.Season, &v.Episode, &v.SeriesName, &v.BlockedID,
		&v.CreatedAt, &v.UpdatedAt, &v.IsFavorite, &v.ProgressSec, &v.ProgressDur)
	v.Blocked = v.BlockedID != ""
	v.HasPoster = v.PosterPath != ""
	v.HasSubtitle = v.SubtitlePath != ""
	if err != nil {
		log.Printf("scan video row: %v", err)
	}
	return v, err
}

func (a *App) listVideos(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}

	args := []any{user.ID}
	// $1 is always present so count and list queries share the same arg layout
	where := []string{"v.available = true AND $1::uuid IS NOT NULL"}
	if q.Get("include_blocked") == "1" && auth.UserFrom(r).Role == "admin" {
		// admins may inspect and unblock blocked videos
		where = append(where, visiblePaths(1))
	} else {
		where = append(where, visibleEpisodes(1))
	}
	argIdx := 2

	if libID := q.Get("library_id"); libID != "" {
		id, err := uuid.Parse(libID)
		if err == nil {
			where = append(where, fmt.Sprintf("v.library_id = $%d", argIdx))
			args = append(args, id)
			argIdx++
		}
	}
	if search := strings.TrimSpace(q.Get("q")); search != "" {
		where = append(where, fmt.Sprintf(
			"(v.title ILIKE $%d OR v.synopsis ILIKE $%d OR v.filename ILIKE $%d OR EXISTS (SELECT 1 FROM unnest(v.genres) g WHERE g ILIKE $%d) OR EXISTS (SELECT 1 FROM video_transcripts vt WHERE vt.video_id = v.id AND vt.status = 'done' AND vt.text ILIKE $%d))",
			argIdx, argIdx, argIdx, argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if genre := strings.TrimSpace(q.Get("genre")); genre != "" {
		where = append(where, fmt.Sprintf("$%d::text = ANY(v.genres)", argIdx))
		args = append(args, genre)
		argIdx++
	}
	if q.Get("favorites") == "true" {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM favorites f2 WHERE f2.user_id=$%d AND f2.video_id=v.id)", argIdx))
		args = append(args, user.ID)
		argIdx++
	}
	if vtype := q.Get("type"); vtype == "tv" {
		where = append(where, "v.series_id IS NOT NULL")
	} else if vtype == "movie" {
		where = append(where, "v.series_id IS NULL")
	}

	orderBy := "lower(v.title), v.year DESC"
	switch q.Get("sort") {
	case "year_desc":
		orderBy = "v.year DESC, lower(v.title)"
	case "year_asc":
		orderBy = "v.year ASC NULLS FIRST, lower(v.title)"
	case "duration_desc":
		orderBy = "v.duration_sec DESC"
	case "added_desc":
		orderBy = "v.created_at DESC"
	case "updated_desc":
		orderBy = "v.updated_at DESC"
	case "favorites_desc":
		orderBy = "(SELECT count(*) FROM favorites fc WHERE fc.video_id=v.id) DESC"
	}

	whereSQL := strings.Join(where, " AND ")
	countSQL := fmt.Sprintf(`SELECT count(*) FROM videos v JOIN libraries l ON l.id=v.library_id WHERE %s`, whereSQL)
	var total int64
	if err := a.pool.QueryRow(r.Context(), countSQL, args...).Scan(&total); err != nil {
		log.Printf("count videos failed: %v", err)
		writeErr(w, http.StatusInternalServerError, "count videos failed")
		return
	}

	args = append(args, pageSize, (page-1)*pageSize)
	sql := fmt.Sprintf(`SELECT %s
		FROM videos v JOIN libraries l ON l.id=v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, videoColumns, blockedLateral, whereSQL, orderBy, argIdx, argIdx+1)

	rows, err := a.pool.Query(r.Context(), sql, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query videos failed")
		return
	}
	defer rows.Close()

	items := []models.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan video row failed")
			return
		}
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, videoListResponse{Items: items, Total: total, Page: page, PageSize: pageSize})
}

func (a *App) getVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	v, err := scanVideo(a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT %s FROM videos v JOIN libraries l ON l.id=v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		%s
		WHERE v.id=$2 AND %s`,
		videoColumns, blockedLateral, visibleEpisodes(1)), auth.UserFrom(r).ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query video failed")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type updateVideoRequest struct {
	Title    string   `json:"title"`
	Synopsis string   `json:"synopsis"`
	Year     int      `json:"year"`
	Genres   []string `json:"genres"`
}

func (a *App) updateVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req updateVideoRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title cannot be empty")
		return
	}
	tag, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET title=$1, synopsis=$2, year=$3, genres=$4, updated_at=now() WHERE id=$5`,
		req.Title, req.Synopsis, req.Year, req.Genres, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

func (a *App) uploadPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var oldPath string
	err = a.pool.QueryRow(r.Context(), `SELECT poster_path FROM videos WHERE id=$1`, id).Scan(&oldPath)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load video failed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	file, header, err := r.FormFile("poster")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing poster file (multipart field 'poster')")
		return
	}
	defer func() { _ = file.Close() }()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ct := http.DetectContentType(head[:n])
	ext := ""
	switch ct {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		writeErr(w, http.StatusBadRequest, "only jpg/png/webp images are supported")
		return
	}
	_ = header

	posterDir := filepath.Join(a.cfg.DataDir, "posters")
	if err := os.MkdirAll(posterDir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot create poster dir")
		return
	}
	dst := filepath.Join(posterDir, id.String()+ext)
	out, err := os.Create(dst)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot save poster")
		return
	}
	defer func() { _ = out.Close() }()
	if _, err := out.Write(head[:n]); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot write poster")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot write poster")
		return
	}
	if oldPath != "" && oldPath != dst {
		_ = os.Remove(oldPath)
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE videos SET poster_path=$1, updated_at=now() WHERE id=$2`, dst, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update video failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "poster updated"})
}

func (a *App) streamVideo(w http.ResponseWriter, r *http.Request) {
	path, ok := a.videoFileFor(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), "inline")
}

func (a *App) downloadVideo(w http.ResponseWriter, r *http.Request) {
	path, ok := a.videoFileFor(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	name := filepath.Base(path)
	disposition := fmt.Sprintf("attachment; filename=%q", mime.QEncoding.Encode("UTF-8", name))
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), disposition)
}

func (a *App) videoFileFor(r *http.Request) (string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return "", false
	}
	var path string
	var available bool
	err = a.pool.QueryRow(r.Context(),
		`SELECT file_path, available FROM videos WHERE id=$1`, id).Scan(&path, &available)
	if err != nil || !available {
		return "", false
	}
	return path, true
}

func (a *App) servePoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(), `SELECT poster_path FROM videos WHERE id=$1`, id).Scan(&path)
	if err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, path)
}

func (a *App) subtitles(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(), `SELECT subtitle_path FROM videos WHERE id=$1`, id).Scan(&path)
	if err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "subtitles not found")
		return
	}
	serveSubtitleFile(w, r, path)
}

func serveSubtitleFile(w http.ResponseWriter, r *http.Request, path string) {
	serveSubtitleFileOffset(w, r, path, 0)
}

// serveSubtitleFileOffset serves a subtitle file as WebVTT, optionally shifting
// every cue by offsetMs so players can sync subtitles without re-encoding.
func serveSubtitleFileOffset(w http.ResponseWriter, r *http.Request, path string, offsetMs int64) {
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "subtitle file unreadable")
		return
	}
	if strings.EqualFold(filepath.Ext(path), ".srt") {
		data = srtToVTT(data)
	}
	if offsetMs != 0 {
		data = shiftVTT(data, offsetMs)
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

type hlsVideo struct {
	FilePath     string
	Width        int
	Height       int
	SubtitlePath string
	Available    bool
	Subs         []media.HLSSubtitle
	Audios       []media.HLSAudio
}

func (a *App) videoForHLS(ctx context.Context, id uuid.UUID) (hlsVideo, bool) {
	var v hlsVideo
	err := a.pool.QueryRow(ctx,
		`SELECT file_path, width, height, subtitle_path, available FROM videos WHERE id=$1`, id,
	).Scan(&v.FilePath, &v.Width, &v.Height, &v.SubtitlePath, &v.Available)
	if err != nil || !v.Available {
		return hlsVideo{}, false
	}
	rows, err := a.pool.Query(ctx, `
		SELECT id::text, lang, title, path FROM subtitle_tracks
		WHERE video_id=$1 ORDER BY position`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var trackID, lang, title, path string
			if err := rows.Scan(&trackID, &lang, &title, &path); err != nil {
				continue
			}
			if fmt := subtitleFormat(path); fmt == "ass" || fmt == "ssa" {
				// ASS tracks are rendered by the player with libass (jassub)
				// to preserve styling; hls.js cannot parse them, so they stay
				// out of the HLS subtitle group.
				continue
			}
			name := title
			if name == "" {
				name = lang
			}
			v.Subs = append(v.Subs, media.HLSSubtitle{
				ID:     trackID,
				Name:   name,
				Active: path != "" && path == v.SubtitlePath,
			})
		}
	}
	if streams, err := media.ProbeAudioStreams(ctx, a.ffprobeBin(), v.FilePath); err == nil {
		for _, s := range streams {
			name := s.Title
			if name == "" {
				name = s.Language
			}
			v.Audios = append(v.Audios, media.HLSAudio{Index: s.Index, Name: name})
		}
	}
	return v, true
}

func (a *App) hlsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	v, ok := a.videoForHLS(r.Context(), id)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	a.serveHLS(w, r, id, v, r.PathValue("file"))
}

// serveHLS serves master / variant / subtitle playlists and HLS segments. It
// is shared by the authed route and the public share route.
func (a *App) serveHLS(w http.ResponseWriter, r *http.Request, id uuid.UUID, v hlsVideo, file string) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}

	if file == "playlist.m3u8" {
		start := 0.0
		if s := r.URL.Query().Get("start"); s != "" {
			start, _ = strconv.ParseFloat(s, 64)
		}
		manifest, err := a.hls.Playlist(r.Context(), id, v.FilePath, start, v.Width, v.Height, v.Subs, v.Audios...)
		if err != nil {
			log.Printf("[hls] %v", err)
			writeErr(w, http.StatusInternalServerError, "failed to start transcode session: "+err.Error())
			return
		}
		data, err := os.ReadFile(manifest)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "read manifest failed")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(rewriteManifest(data, token))
		return
	}

	if strings.HasPrefix(file, "subs/") {
		parts := strings.Split(file, "/")
		if len(parts) == 3 && parts[0] == "subs" {
			trackID, err := uuid.Parse(parts[1])
			if err == nil {
				switch parts[2] {
				case "playlist.m3u8":
					if _, ok := a.loadSubtitleTrack(r.Context(), id, trackID); ok {
						w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
						w.Header().Set("Cache-Control", "no-store")
						_, _ = w.Write(buildSubtitlePlaylist(token))
						return
					}
				case "subtitle.vtt":
					t, ok := a.loadSubtitleTrack(r.Context(), id, trackID)
					if ok {
						if path, err := a.ensureSubtitlePath(r.Context(), id, &t); err == nil {
							serveSubtitleFile(w, r, path)
							return
						}
					}
				}
			}
		}
		writeErr(w, http.StatusNotFound, "subtitles not found")
		return
	}

	path, ok := a.hls.SessionFile(id, file)
	if !ok {
		writeErr(w, http.StatusNotFound, "segment not found")
		return
	}
	if strings.HasSuffix(file, ".m3u8") {
		data, err := os.ReadFile(path)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "read manifest failed")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(rewriteManifest(data, token))
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	http.ServeFile(w, r, path)
}

func buildSubtitlePlaylist(token string) []byte {
	return []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:99999\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:214748.000000,\nsubtitle.vtt?token=" + url.QueryEscape(token) + "\n#EXT-X-ENDLIST\n")
}

func rewriteManifest(data []byte, token string) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines[i] = trimmed + "?token=" + url.QueryEscape(token)
	}
	return []byte(strings.Join(lines, "\n"))
}

func (a *App) scrapeVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	provider := r.URL.Query().Get("provider")
	force := r.URL.Query().Get("force") == "1"
	if err := a.scraper.ScrapeWith(r.Context(), id, provider, force); err != nil {
		if strings.Contains(err.Error(), "already has metadata") {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "scrape complete"})
}

func srtToVTT(data []byte) []byte {
	s := string(data)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, line := range lines {
		if strings.Contains(line, " --> ") {
			parts := strings.Split(line, " --> ")
			line = strings.ReplaceAll(parts[0], ",", ".") + " --> " + strings.ReplaceAll(parts[1], ",", ".")
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// parseVTTTime parses a WebVTT timestamp (H:MM:SS.mmm, MM:SS.mmm or SS.mmm).
func parseVTTTime(t string) (float64, bool) {
	t = strings.TrimSpace(t)
	parts := strings.Split(t, ":")
	if len(parts) == 3 {
		h, e1 := strconv.Atoi(parts[0])
		m, e2 := strconv.Atoi(parts[1])
		s, e3 := strconv.ParseFloat(parts[2], 64)
		if e1 != nil || e2 != nil || e3 != nil {
			return 0, false
		}
		return float64(h)*3600 + float64(m)*60 + s, true
	}
	if len(parts) == 2 {
		m, e1 := strconv.Atoi(parts[0])
		s, e2 := strconv.ParseFloat(parts[1], 64)
		if e1 != nil || e2 != nil {
			return 0, false
		}
		return float64(m)*60 + s, true
	}
	if len(parts) == 1 {
		s, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, false
		}
		return s, true
	}
	return 0, false
}

func formatVTTTime(sec float64) string {
	sec = math.Max(0, sec)
	h := int(sec / 3600)
	m := int(sec/60) % 60
	s := sec - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, s)
}

// shiftVTT moves every cue timing in a WebVTT document by offsetMs. Non-timing
// lines and cue settings after the arrow are preserved.
func shiftVTT(data []byte, offsetMs int64) []byte {
	if offsetMs == 0 {
		return data
	}
	offset := float64(offsetMs) / 1000
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.Contains(line, " --> ") {
			continue
		}
		parts := strings.SplitN(line, " --> ", 2)
		start, ok1 := parseVTTTime(parts[0])
		rest := ""
		endPart := parts[1]
		if idx := strings.Index(endPart, " "); idx >= 0 {
			rest = endPart[idx:]
			endPart = endPart[:idx]
		}
		end, ok2 := parseVTTTime(endPart)
		if !ok1 || !ok2 {
			continue
		}
		lines[i] = formatVTTTime(start+offset) + " --> " + formatVTTTime(end+offset) + rest
	}
	return []byte(strings.Join(lines, "\n"))
}
