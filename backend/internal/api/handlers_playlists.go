package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

func (a *App) listPlaylists(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), `
		SELECT p.id, p.user_id, p.name, p.description, p.created_at, p.updated_at,
		       (SELECT count(*) FROM playlist_items pi WHERE pi.playlist_id=p.id)
		FROM playlists p WHERE p.user_id=$1
		ORDER BY p.updated_at DESC`, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query playlists failed")
		return
	}
	defer rows.Close()

	items := []models.Playlist{}
	for rows.Next() {
		var p models.Playlist
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description,
			&p.CreatedAt, &p.UpdatedAt, &p.ItemCount); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan row failed")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type playlistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (a *App) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var req playlistRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	user := auth.UserFrom(r)
	p := models.Playlist{UserID: user.ID, Name: req.Name, Description: req.Description}
	err := a.pool.QueryRow(r.Context(),
		`INSERT INTO playlists (user_id, name, description) VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		user.ID, req.Name, req.Description).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create playlist failed")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *App) getPlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	user := auth.UserFrom(r)
	var p models.Playlist
	err = a.pool.QueryRow(r.Context(),
		`SELECT id, user_id, name, description, created_at, updated_at FROM playlists WHERE id=$1 AND user_id=$2`,
		id, user.ID).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "playlist not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load playlist failed")
		return
	}

	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT pi.position, pi.added_at, %s
		FROM playlist_items pi
		JOIN videos v ON v.id = pi.video_id
		JOIN libraries l ON l.id = v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		%s
		WHERE pi.playlist_id=$2 AND `+visibleEpisodes(1)+` AND `+primaryVersionCondition()+`
		ORDER BY pi.position, pi.added_at`, videoColumns, blockedLateral), user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load playlist items failed")
		return
	}
	defer rows.Close()

	items := []models.PlaylistItem{}
	for rows.Next() {
		var it models.PlaylistItem
		if err := rows.Scan(&it.Position, &it.AddedAt, &it.Video.ID, &it.Video.LibraryID,
			&it.Video.LibraryName, &it.Video.Title, &it.Video.Filename, &it.Video.FilePath,
			&it.Video.SizeBytes, &it.Video.DurationSec, &it.Video.Width, &it.Video.Height,
			&it.Video.VideoCodec, &it.Video.Container, &it.Video.Year, &it.Video.Synopsis,
			&it.Video.Genres, &it.Video.PosterPath, &it.Video.SubtitlePath, &it.Video.Available,
			&it.Video.SeriesID, &it.Video.Season, &it.Video.Episode, &it.Video.SeriesName,
			&it.Video.VersionKey, &it.Video.VersionLabel, &it.Video.VersionCount, &it.Video.IsPrimary,
			&it.Video.CreatedAt, &it.Video.UpdatedAt, &it.Video.IsFavorite,
			&it.Video.ProgressSec, &it.Video.ProgressDur); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan item failed")
			return
		}
		it.Video.HasPoster = it.Video.PosterPath != ""
		it.Video.HasSubtitle = it.Video.SubtitlePath != ""
		items = append(items, it)
	}
	p.ItemCount = len(items)
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p, "items": items})
}

func (a *App) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	var req playlistRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := auth.UserFrom(r)
	tag, err := a.pool.Exec(r.Context(),
		`UPDATE playlists SET name=$1, description=$2, updated_at=now() WHERE id=$3 AND user_id=$4`,
		strings.TrimSpace(req.Name), req.Description, id, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update playlist failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "playlist not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

func (a *App) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM playlists WHERE id=$1 AND user_id=$2`, id, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete playlist failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

func (a *App) addPlaylistItem(w http.ResponseWriter, r *http.Request) {
	playlistID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	var req struct {
		VideoID uuid.UUID `json:"video_id"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := auth.UserFrom(r)

	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1 AND user_id=$2)`,
		playlistID, user.ID).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "playlist not found")
		return
	}
	var videoExists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM videos WHERE id=$1 AND available=true)`,
		req.VideoID).Scan(&videoExists); err != nil || !videoExists {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}

	tag, err := a.pool.Exec(r.Context(), `
		INSERT INTO playlist_items (playlist_id, video_id, position)
		VALUES ($1, $2, COALESCE((SELECT max(position)+1 FROM playlist_items WHERE playlist_id=$1), 1))
		ON CONFLICT (playlist_id, video_id) DO NOTHING`,
		playlistID, req.VideoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "add item failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusConflict, "video already in playlist")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `UPDATE playlists SET updated_at=now() WHERE id=$1`, playlistID)
	writeJSON(w, http.StatusCreated, map[string]any{"message": "added"})
}

func (a *App) removePlaylistItem(w http.ResponseWriter, r *http.Request) {
	playlistID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid playlist id")
		return
	}
	videoID, err := uuid.Parse(r.PathValue("videoId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	user := auth.UserFrom(r)
	tag, err := a.pool.Exec(r.Context(), `
		DELETE FROM playlist_items pi
		USING playlists p
		WHERE pi.playlist_id=p.id AND p.id=$1 AND p.user_id=$2 AND pi.video_id=$3`,
		playlistID, user.ID, videoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "remove item failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `UPDATE playlists SET updated_at=now() WHERE id=$1`, playlistID)
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}
