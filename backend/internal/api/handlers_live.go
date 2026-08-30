package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type liveStream struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	StreamKey string    `json:"stream_key,omitempty"`
	IngestURL string    `json:"ingest_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type createLiveRequest struct {
	Title string `json:"title"`
}

type sendChatRequest struct {
	Body string `json:"body"`
}

type chatMessage struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func newLiveKey() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) loadLiveStream(id uuid.UUID) (liveStream, bool) {
	var s liveStream
	err := a.pool.QueryRow(context.Background(), `
		SELECT id, title, status, error, stream_key, created_at
		FROM live_streams WHERE id=$1`, id).
		Scan(&s.ID, &s.Title, &s.Status, &s.Error, &s.StreamKey, &s.CreatedAt)
	return s, err == nil
}

func (a *App) ingestURLFor(key string) string {
	return strings.TrimSuffix(a.cfg.RTMPIngestURL, "/") + "/" + key
}

// POST /api/live — create a live stream (admin); returns the RTMP ingest URL.
func (a *App) createLiveStream(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req createLiveRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	key, err := newLiveKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "generate stream key failed")
		return
	}
	var s liveStream
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO live_streams (title, stream_key, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, title, status, error, stream_key, created_at`,
		req.Title, key, user.ID).Scan(&s.ID, &s.Title, &s.Status, &s.Error, &s.StreamKey, &s.CreatedAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "create live stream failed")
		return
	}
	s.IngestURL = a.ingestURLFor(s.StreamKey)
	writeJSON(w, http.StatusCreated, s)
}

// GET /api/live — list live streams (most recent first).
func (a *App) listLiveStreams(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, title, status, error, stream_key, created_at
		FROM live_streams ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list live streams failed")
		return
	}
	defer rows.Close()
	items := []liveStream{}
	for rows.Next() {
		var s liveStream
		if err := rows.Scan(&s.ID, &s.Title, &s.Status, &s.Error, &s.StreamKey, &s.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan live stream failed")
			return
		}
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/live/{id} — stream status; ingest details are admin-only.
func (a *App) getLiveStream(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	s, ok := a.loadLiveStream(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "live stream not found")
		return
	}
	user := auth.UserFrom(r)
	if user.Role == "admin" {
		s.IngestURL = a.ingestURLFor(s.StreamKey)
	}
	writeJSON(w, http.StatusOK, s)
}

// POST /api/live/{id}/start — start pulling the RTMP ingest into HLS.
func (a *App) startLiveStream(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	s, ok := a.loadLiveStream(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "live stream not found")
		return
	}
	if s.Status == "live" || s.Status == "starting" {
		writeErr(w, http.StatusConflict, "stream is already running")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE live_streams SET status='starting', error='', updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update live stream failed")
		return
	}
	if err := a.live.Start(context.Background(), id, a.ingestURLFor(s.StreamKey)); err != nil {
		_, _ = a.pool.Exec(r.Context(),
			`UPDATE live_streams SET status='offline', error=$2, updated_at=now() WHERE id=$1`,
			id, err.Error())
		writeErr(w, http.StatusInternalServerError, "start live ingest failed: "+err.Error())
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE live_streams SET status='live', error='', updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update live stream failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "live"})
}

// POST /api/live/{id}/stop — stop the ingest worker.
func (a *App) stopLiveStream(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	a.live.Stop(id)
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE live_streams SET status='idle', updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update live stream failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "idle"})
}

var liveSegmentRe = regexp.MustCompile(`^(index\.m3u8|seg_\d{5}\.ts)$`)

// GET /api/live/{id}/hls/{file...} — HLS playlist/segments for the watch page.
func (a *App) liveHLS(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	file := r.PathValue("file")
	if !liveSegmentRe.MatchString(file) {
		writeErr(w, http.StatusNotFound, "segment not found")
		return
	}
	dir := a.live.StreamDir(id)
	path := filepath.Join(dir, file)
	if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
		writeErr(w, http.StatusNotFound, "segment not found")
		return
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		writeErr(w, http.StatusNotFound, "segment not found")
		return
	}
	if strings.HasSuffix(file, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	http.ServeFile(w, r, path)
}

// GET /api/live/{id}/chat?after=<message-id> — poll messages newer than after.
func (a *App) listChatMessages(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	query := `
		SELECT id, username, body, created_at FROM chat_messages
		WHERE live_id=$1`
	args := []any{id}
	if after := r.URL.Query().Get("after"); after != "" {
		if afterID, err := uuid.Parse(after); err == nil {
			query += ` AND created_at > (SELECT created_at FROM chat_messages WHERE id=$2)`
			args = append(args, afterID)
		}
	}
	query += ` ORDER BY created_at LIMIT 200`
	rows, err := a.pool.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list chat messages failed")
		return
	}
	defer rows.Close()
	items := []chatMessage{}
	for rows.Next() {
		var m chatMessage
		if err := rows.Scan(&m.ID, &m.Username, &m.Body, &m.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan chat message failed")
			return
		}
		items = append(items, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/live/{id}/chat — send a chat message.
func (a *App) sendChatMessage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid live stream id")
		return
	}
	var req sendChatRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len(req.Body) > 500 {
		writeErr(w, http.StatusBadRequest, "message must be 1-500 characters")
		return
	}
	var m chatMessage
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO chat_messages (live_id, user_id, username, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, body, created_at`,
		id, user.ID, user.DisplayName, req.Body).Scan(&m.ID, &m.Username, &m.Body, &m.CreatedAt); err != nil {
		writeErr(w, http.StatusBadRequest, "live stream not found")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}
