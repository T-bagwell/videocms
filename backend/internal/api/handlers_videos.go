package api

import (
	"errors"
	"fmt"
	"io"
	"log"
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
	v.created_at, v.updated_at,
	EXISTS(SELECT 1 FROM favorites f WHERE f.user_id=$1 AND f.video_id=v.id) AS is_fav,
	COALESCE((SELECT wp.position_sec FROM watch_progress wp WHERE wp.user_id=$1 AND wp.video_id=v.id), 0),
	COALESCE((SELECT wp.duration_sec FROM watch_progress wp WHERE wp.user_id=$1 AND wp.video_id=v.id), 0)`

func scanVideo(row pgx.Row) (models.Video, error) {
	var v models.Video
	err := row.Scan(&v.ID, &v.LibraryID, &v.LibraryName, &v.Title, &v.Filename, &v.FilePath,
		&v.SizeBytes, &v.DurationSec, &v.Width, &v.Height, &v.VideoCodec, &v.Container,
		&v.Year, &v.Synopsis, &v.Genres, &v.PosterPath, &v.SubtitlePath, &v.Available,
		&v.SeriesID, &v.Season, &v.Episode, &v.SeriesName,
		&v.CreatedAt, &v.UpdatedAt, &v.IsFavorite, &v.ProgressSec, &v.ProgressDur)
	v.HasPoster = v.PosterPath != ""
	v.HasSubtitle = v.SubtitlePath != ""
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
			"(v.title ILIKE $%d OR v.synopsis ILIKE $%d OR v.filename ILIKE $%d OR EXISTS (SELECT 1 FROM unnest(v.genres) g WHERE g ILIKE $%d))",
			argIdx, argIdx, argIdx, argIdx))
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
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, videoColumns, whereSQL, orderBy, argIdx, argIdx+1)

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
		WHERE v.id=$2`,
		videoColumns), auth.UserFrom(r).ID, id))
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
	defer file.Close()

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
	defer out.Close()
	if _, err := out.Write(head[:n]); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot write poster")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot write poster")
		return
	}
	if oldPath != "" && oldPath != dst {
		os.Remove(oldPath)
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
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "subtitle file unreadable")
		return
	}
	if strings.EqualFold(filepath.Ext(path), ".srt") {
		data = srtToVTT(data)
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

func (a *App) hlsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	file := r.PathValue("file")
	if file == "playlist.m3u8" {
		input, ok := a.videoFileFor(r)
		if !ok {
			writeErr(w, http.StatusNotFound, "video not found or unavailable")
			return
		}
		start := 0.0
		if s := r.URL.Query().Get("start"); s != "" {
			start, _ = strconv.ParseFloat(s, 64)
		}
		manifest, err := a.hls.Playlist(r.Context(), id, input, start)
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
		token := r.URL.Query().Get("token")
		if token == "" {
			token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(rewriteManifest(data, token))
		return
	}

	path, ok := a.hls.Segment(id, file)
	if !ok {
		writeErr(w, http.StatusNotFound, "segment not found")
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	http.ServeFile(w, r, path)
}

func rewriteManifest(data []byte, token string) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if segmentNameRe(trimmed) {
			lines[i] = trimmed + "?token=" + url.QueryEscape(token)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func segmentNameRe(name string) bool {
	return media.SegmentNameMatch(name)
}

func (a *App) scrapeVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	if err := a.scraper.Scrape(r.Context(), id); err != nil {
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
