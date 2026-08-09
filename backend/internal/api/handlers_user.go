package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

type saveProgressRequest struct {
	VideoID     uuid.UUID `json:"video_id"`
	PositionSec float64   `json:"position_sec"`
	DurationSec float64   `json:"duration_sec"`
}

func (a *App) saveProgress(w http.ResponseWriter, r *http.Request) {
	var req saveProgressRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PositionSec < 0 {
		req.PositionSec = 0
	}
	user := auth.UserFrom(r)
	tag, err := a.pool.Exec(r.Context(), `
		INSERT INTO watch_progress (user_id, video_id, position_sec, duration_sec, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (user_id, video_id) DO UPDATE SET
			position_sec = EXCLUDED.position_sec,
			duration_sec = EXCLUDED.duration_sec,
			updated_at = now()`,
		user.ID, req.VideoID, req.PositionSec, req.DurationSec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save progress failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "saved"})
}

func (a *App) continueWatching(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), `
		SELECT v.id, v.library_id, l.name, v.title, v.filename, v.file_path, v.size_bytes,
		       v.duration_sec, v.width, v.height, v.video_codec, v.container, v.year,
		       v.synopsis, v.genres, v.poster_path, v.subtitle_path, v.available,
		       v.series_id, v.season, v.episode, COALESCE(s.name, ''),
		       v.created_at, v.updated_at,
		       EXISTS(SELECT 1 FROM favorites f WHERE f.user_id=$1 AND f.video_id=v.id),
		       wp.position_sec, wp.duration_sec
		FROM watch_progress wp
		JOIN videos v ON v.id = wp.video_id
		JOIN libraries l ON l.id = v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		WHERE wp.user_id=$1 AND v.available=true AND wp.position_sec > 5
		  AND wp.position_sec < wp.duration_sec * 0.95
		ORDER BY wp.updated_at DESC
		LIMIT 16`, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query continue watching failed")
		return
	}
	defer rows.Close()

	items := []models.Video{}
	for rows.Next() {
		var v models.Video
		if err := rows.Scan(&v.ID, &v.LibraryID, &v.LibraryName, &v.Title, &v.Filename, &v.FilePath,
			&v.SizeBytes, &v.DurationSec, &v.Width, &v.Height, &v.VideoCodec, &v.Container,
			&v.Year, &v.Synopsis, &v.Genres, &v.PosterPath, &v.SubtitlePath, &v.Available,
			&v.SeriesID, &v.Season, &v.Episode, &v.SeriesName,
			&v.CreatedAt, &v.UpdatedAt, &v.IsFavorite, &v.ProgressSec, &v.ProgressDur); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan row failed")
			return
		}
		v.HasPoster = v.PosterPath != ""
		v.HasSubtitle = v.SubtitlePath != ""
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) addFavorite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VideoID uuid.UUID `json:"video_id"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := auth.UserFrom(r)
	tag, err := a.pool.Exec(r.Context(),
		`INSERT INTO favorites (user_id, video_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		user.ID, req.VideoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "add favorite failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusConflict, "already in favorites")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": "favorited"})
}

func (a *App) removeFavorite(w http.ResponseWriter, r *http.Request) {
	videoID, err := uuid.Parse(r.PathValue("videoId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM favorites WHERE user_id=$1 AND video_id=$2`, user.ID, videoID); err != nil {
		writeErr(w, http.StatusInternalServerError, "remove favorite failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}

func (a *App) listFavorites(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM favorites f
		JOIN videos v ON v.id = f.video_id
		JOIN libraries l ON l.id = v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		WHERE f.user_id=$2 AND v.available=true
		ORDER BY f.created_at DESC`, videoColumns), user.ID, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query favorites failed")
		return
	}
	defer rows.Close()

	items := []models.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan row failed")
			return
		}
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
