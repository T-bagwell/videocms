package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type watchRoom struct {
	ID          uuid.UUID `json:"id"`
	Token       string    `json:"token"`
	VideoID     uuid.UUID `json:"video_id"`
	VideoTitle  string    `json:"video_title,omitempty"`
	DurationSec float64   `json:"duration_sec,omitempty"`
	Playing     bool      `json:"playing"`
	PositionSec float64   `json:"position_sec"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type createWatchRoomRequest struct {
	VideoID uuid.UUID `json:"video_id"`
}

type updateWatchRoomRequest struct {
	Token       string  `json:"token"`
	Playing     bool    `json:"playing"`
	PositionSec float64 `json:"position_sec"`
}

func newWatchRoomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) loadWatchRoom(id uuid.UUID) (watchRoom, error) {
	var r watchRoom
	err := a.pool.QueryRow(context.Background(), `
		SELECT id, token, video_id, playing, position_sec, updated_at, created_at
		FROM watch_rooms WHERE id=$1`, id).
		Scan(&r.ID, &r.Token, &r.VideoID, &r.Playing, &r.PositionSec, &r.UpdatedAt, &r.CreatedAt)
	return r, err
}

// POST /api/watch/rooms — create a synchronized watch room for a video.
func (a *App) createWatchRoom(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req createWatchRoomRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var title string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT title FROM videos WHERE id=$1`, req.VideoID).Scan(&title); err != nil {
		writeErr(w, http.StatusBadRequest, "video not found")
		return
	}
	token, err := newWatchRoomToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "generate room token failed")
		return
	}
	var room watchRoom
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO watch_rooms (token, video_id, owner_id)
		VALUES ($1, $2, $3)
		RETURNING id, token, video_id, playing, position_sec, updated_at, created_at`,
		token, req.VideoID, user.ID).Scan(
		&room.ID, &room.Token, &room.VideoID, &room.Playing, &room.PositionSec, &room.UpdatedAt, &room.CreatedAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "create watch room failed")
		return
	}
	room.VideoTitle = title
	writeJSON(w, http.StatusCreated, room)
}

func (a *App) watchRoomFromRequest(w http.ResponseWriter, r *http.Request) (watchRoom, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid room id")
		return watchRoom{}, false
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		writeErr(w, http.StatusBadRequest, "room token is required")
		return watchRoom{}, false
	}
	room, err := a.loadWatchRoom(id)
	if err != nil || room.Token != token {
		writeErr(w, http.StatusForbidden, "invalid room token")
		return watchRoom{}, false
	}
	return room, true
}

// GET /api/watch/rooms/{id}?token=… — current synchronized state.
func (a *App) getWatchRoom(w http.ResponseWriter, r *http.Request) {
	room, ok := a.watchRoomFromRequest(w, r)
	if !ok {
		return
	}
	var title string
	var dur float64
	_ = a.pool.QueryRow(r.Context(),
		`SELECT title, duration_sec FROM videos WHERE id=$1`, room.VideoID).Scan(&title, &dur)
	room.VideoTitle = title
	room.DurationSec = dur
	writeJSON(w, http.StatusOK, room)
}

// POST /api/watch/rooms/{id}/join — validate a token and return room state.
func (a *App) joinWatchRoom(w http.ResponseWriter, r *http.Request) {
	room, ok := a.watchRoomFromRequest(w, r)
	if !ok {
		return
	}
	var title string
	var dur float64
	_ = a.pool.QueryRow(r.Context(),
		`SELECT title, duration_sec FROM videos WHERE id=$1`, room.VideoID).Scan(&title, &dur)
	room.VideoTitle = title
	room.DurationSec = dur
	writeJSON(w, http.StatusOK, room)
}

// PUT /api/watch/rooms/{id} — publish playback state (any authenticated member).
func (a *App) updateWatchRoom(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid room id")
		return
	}
	var req updateWatchRoomRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var room watchRoom
	err = a.pool.QueryRow(r.Context(), `
		UPDATE watch_rooms
		SET playing=$2, position_sec=$3, updated_at=now()
		WHERE id=$1 AND token=$4
		RETURNING id, token, video_id, playing, position_sec, updated_at, created_at`,
		id, req.Playing, req.PositionSec, req.Token).Scan(
		&room.ID, &room.Token, &room.VideoID, &room.Playing, &room.PositionSec, &room.UpdatedAt, &room.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusForbidden, "invalid room token")
		return
	}
	writeJSON(w, http.StatusOK, room)
}
