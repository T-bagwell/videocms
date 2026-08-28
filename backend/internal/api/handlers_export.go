package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

// queryRowsAny runs a query and returns every row as a JSON-friendly map keyed
// by column name.
func (a *App) queryRowsAny(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := rows.FieldDescriptions()
	items := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([16]byte); ok {
				if u, err := uuid.FromBytes(b[:]); err == nil {
					v = u.String()
				}
			}
			row[c.Name] = v
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func serveJSONDownload(w http.ResponseWriter, name string, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// exportAll dumps the full metadata backup (users, libraries, videos, series,
// playlists, personal data and content controls). Passwords are never
// included.
// GET /api/admin/export
func (a *App) exportAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := map[string]any{"exported_at": time.Now().UTC()}
	var err error
	if out["users"], err = a.queryRowsAny(ctx,
		`SELECT id, username, role, display_name, created_at FROM users ORDER BY username`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export users failed")
		return
	}
	if out["libraries"], err = a.queryRowsAny(ctx,
		`SELECT id, name, path, blocked, video_count, created_at FROM libraries ORDER BY name`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export libraries failed")
		return
	}
	if out["videos"], err = a.queryRowsAny(ctx, `
		SELECT id, library_id, title, filename, file_path, size_bytes, duration_sec,
		       width, height, video_codec, container, year, synopsis, genres,
		       tmdb_id, scraped_at, available, created_at, updated_at
		FROM videos ORDER BY title`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export videos failed")
		return
	}
	if out["series"], err = a.queryRowsAny(ctx,
		`SELECT id, library_id, name, season, episode_count, updated_at FROM series ORDER BY name, season`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export series failed")
		return
	}
	if out["playlists"], err = a.queryRowsAny(ctx,
		`SELECT id, user_id, name, description, created_at, updated_at FROM playlists ORDER BY name`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export playlists failed")
		return
	}
	if out["playlist_items"], err = a.queryRowsAny(ctx,
		`SELECT playlist_id, video_id, position FROM playlist_items ORDER BY playlist_id, position`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export playlist items failed")
		return
	}
	if out["favorites"], err = a.queryRowsAny(ctx,
		`SELECT user_id, video_id, created_at FROM favorites ORDER BY user_id`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export favorites failed")
		return
	}
	if out["series_favorites"], err = a.queryRowsAny(ctx,
		`SELECT user_id, series_id, created_at FROM series_favorites ORDER BY user_id`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export series favorites failed")
		return
	}
	if out["watch_progress"], err = a.queryRowsAny(ctx,
		`SELECT user_id, video_id, position_sec, duration_sec, updated_at FROM watch_progress ORDER BY user_id`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export watch progress failed")
		return
	}
	if out["hidden_paths"], err = a.queryRowsAny(ctx,
		`SELECT id, user_id, path, created_at FROM hidden_paths ORDER BY user_id`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export hidden paths failed")
		return
	}
	if out["blocked_titles"], err = a.queryRowsAny(ctx,
		`SELECT id, title, created_at FROM blocked_titles ORDER BY title`); err != nil {
		writeErr(w, http.StatusInternalServerError, "export blocked titles failed")
		return
	}
	serveJSONDownload(w, "videocms-backup-"+time.Now().Format("20060102-150405")+".json", out)
}

// exportMe dumps the current user's personal data: favorites, series
// favorites, watch progress, hidden paths and playlists.
// GET /api/users/me/export
func (a *App) exportMe(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	ctx := r.Context()
	out := map[string]any{
		"exported_at": time.Now().UTC(),
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
		},
	}
	var err error
	if out["favorites"], err = a.queryRowsAny(ctx,
		`SELECT video_id, created_at FROM favorites WHERE user_id=$1 ORDER BY created_at`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export favorites failed")
		return
	}
	if out["series_favorites"], err = a.queryRowsAny(ctx,
		`SELECT series_id, created_at FROM series_favorites WHERE user_id=$1 ORDER BY created_at`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export series favorites failed")
		return
	}
	if out["watch_progress"], err = a.queryRowsAny(ctx,
		`SELECT video_id, position_sec, duration_sec, updated_at FROM watch_progress WHERE user_id=$1 ORDER BY updated_at`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export watch progress failed")
		return
	}
	if out["hidden_paths"], err = a.queryRowsAny(ctx,
		`SELECT id, path, created_at FROM hidden_paths WHERE user_id=$1 ORDER BY created_at`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export hidden paths failed")
		return
	}
	if out["playlists"], err = a.queryRowsAny(ctx,
		`SELECT id, name, description, created_at FROM playlists WHERE user_id=$1 ORDER BY name`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export playlists failed")
		return
	}
	if out["playlist_items"], err = a.queryRowsAny(ctx, `
		SELECT pi.playlist_id, pi.video_id, pi.position
		FROM playlist_items pi
		JOIN playlists p ON p.id = pi.playlist_id
		WHERE p.user_id=$1 ORDER BY pi.playlist_id, pi.position`, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "export playlist items failed")
		return
	}
	serveJSONDownload(w, "videocms-my-data-"+time.Now().Format("20060102-150405")+".json", out)
}

type backupUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

type backupLibrary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Blocked bool   `json:"blocked"`
}

type backupVideo struct {
	ID          string   `json:"id"`
	LibraryID   string   `json:"library_id"`
	Title       string   `json:"title"`
	Filename    string   `json:"filename"`
	FilePath    string   `json:"file_path"`
	SizeBytes   int64    `json:"size_bytes"`
	DurationSec float64  `json:"duration_sec"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	VideoCodec  string   `json:"video_codec"`
	Container   string   `json:"container"`
	Year        int      `json:"year"`
	Synopsis    string   `json:"synopsis"`
	Genres      []string `json:"genres"`
	TmdbID      int      `json:"tmdb_id"`
	Available   bool     `json:"available"`
	SeriesID    string   `json:"series_id,omitempty"`
	Season      int      `json:"season,omitempty"`
	Episode     int      `json:"episode,omitempty"`
}

type backupSeries struct {
	ID           string `json:"id"`
	LibraryID    string `json:"library_id"`
	Name         string `json:"name"`
	Season       int    `json:"season"`
	EpisodeCount int    `json:"episode_count"`
}

type backupPlaylist struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type backupPlaylistItem struct {
	PlaylistID string `json:"playlist_id"`
	VideoID    string `json:"video_id"`
	Position   int    `json:"position"`
}

type backupPair struct {
	UserID  string `json:"user_id"`
	VideoID string `json:"video_id"`
}

type backupSeriesFav struct {
	UserID   string `json:"user_id"`
	SeriesID string `json:"series_id"`
}

type backupProgress struct {
	UserID      string  `json:"user_id"`
	VideoID     string  `json:"video_id"`
	PositionSec float64 `json:"position_sec"`
	DurationSec float64 `json:"duration_sec"`
}

type backupHiddenPath struct {
	UserID string `json:"user_id"`
	Path   string `json:"path"`
}

type backupBlockedTitle struct {
	Title string `json:"title"`
}

type backupFile struct {
	Users           []backupUser         `json:"users"`
	Libraries       []backupLibrary      `json:"libraries"`
	Videos          []backupVideo        `json:"videos"`
	Series          []backupSeries       `json:"series"`
	Playlists       []backupPlaylist     `json:"playlists"`
	PlaylistItems   []backupPlaylistItem `json:"playlist_items"`
	Favorites       []backupPair         `json:"favorites"`
	SeriesFavorites []backupSeriesFav    `json:"series_favorites"`
	WatchProgress   []backupProgress     `json:"watch_progress"`
	HiddenPaths     []backupHiddenPath   `json:"hidden_paths"`
	BlockedTitles   []backupBlockedTitle `json:"blocked_titles"`
}

// importBackup restores a backup produced by /api/admin/export. Libraries are
// upserted by path, videos by file path, series by (library, name, season);
// personal data is restored for users that already exist. Files are never
// copied — only the metadata index is rebuilt.
// POST /api/admin/import (multipart field "backup")
func (a *App) importBackup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, _, err := r.FormFile("backup")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing backup file")
		return
	}
	defer file.Close()
	var backup backupFile
	if err := json.NewDecoder(io.LimitReader(file, 256<<20)).Decode(&backup); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid backup JSON: "+err.Error())
		return
	}

	ctx := r.Context()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "begin transaction failed")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	counts := map[string]int{}
	fail := func(msg string) {
		writeErr(w, http.StatusInternalServerError, msg)
	}

	libMap := map[string]uuid.UUID{}
	for _, lib := range backup.Libraries {
		if lib.Path == "" {
			continue
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO libraries (name, path, blocked) VALUES ($1,$2,$3)
			ON CONFLICT (path) DO UPDATE SET name=EXCLUDED.name, blocked=EXCLUDED.blocked
			RETURNING id`, lib.Name, lib.Path, lib.Blocked).Scan(&id); err != nil {
			fail("import libraries failed: " + err.Error())
			return
		}
		libMap[lib.ID] = id
		counts["libraries"]++
	}

	videoMap := map[string]uuid.UUID{}
	for _, v := range backup.Videos {
		if v.FilePath == "" {
			continue
		}
		libID, ok := libMap[v.LibraryID]
		if !ok {
			continue
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO videos (library_id, title, filename, file_path, size_bytes, duration_sec,
			                    width, height, video_codec, container, year, synopsis, genres,
			                    tmdb_id, available)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT (file_path) DO UPDATE SET
				library_id=EXCLUDED.library_id, title=EXCLUDED.title, filename=EXCLUDED.filename,
				size_bytes=EXCLUDED.size_bytes, duration_sec=EXCLUDED.duration_sec,
				width=EXCLUDED.width, height=EXCLUDED.height, video_codec=EXCLUDED.video_codec,
				container=EXCLUDED.container, year=EXCLUDED.year, synopsis=EXCLUDED.synopsis,
				genres=EXCLUDED.genres, tmdb_id=EXCLUDED.tmdb_id, available=EXCLUDED.available,
				updated_at=now()
			RETURNING id`,
			libID, v.Title, v.Filename, v.FilePath, v.SizeBytes, v.DurationSec,
			v.Width, v.Height, v.VideoCodec, v.Container, v.Year, v.Synopsis, v.Genres,
			v.TmdbID, v.Available).Scan(&id); err != nil {
			fail("import videos failed: " + err.Error())
			return
		}
		videoMap[v.ID] = id
		counts["videos"]++
	}

	seriesMap := map[string]uuid.UUID{}
	for _, s := range backup.Series {
		libID, ok := libMap[s.LibraryID]
		if !ok {
			continue
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO series (library_id, name, season, episode_count) VALUES ($1,$2,$3,$4)
			ON CONFLICT (library_id, name, season)
			DO UPDATE SET episode_count=EXCLUDED.episode_count, updated_at=now()
			RETURNING id`, libID, s.Name, s.Season, s.EpisodeCount).Scan(&id); err != nil {
			fail("import series failed: " + err.Error())
			return
		}
		seriesMap[s.ID] = id
		counts["series"]++
	}

	for _, v := range backup.Videos {
		newID, ok1 := videoMap[v.ID]
		seriesID, ok2 := seriesMap[v.SeriesID]
		if !ok1 || !ok2 || v.SeriesID == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE videos SET series_id=$1, season=$2, episode=$3 WHERE id=$4`,
			seriesID, v.Season, v.Episode, newID); err != nil {
			log.Printf("import: attach series: %v", err)
		}
	}

	userMap := map[string]uuid.UUID{}
	for _, u := range backup.Users {
		var id uuid.UUID
		if err := tx.QueryRow(ctx,
			`SELECT id FROM users WHERE username=$1`, u.Username).Scan(&id); err != nil {
			continue // user does not exist locally; their personal data is skipped
		}
		userMap[u.ID] = id
	}

	for _, b := range backup.BlockedTitles {
		if b.Title == "" {
			continue
		}
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM blocked_titles WHERE title=$1)`, b.Title).Scan(&exists); err != nil || exists {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO blocked_titles (title) VALUES ($1)`, b.Title); err == nil {
			counts["blocked_titles"]++
		}
	}

	for _, f := range backup.Favorites {
		uid, ok1 := userMap[f.UserID]
		vid, ok2 := videoMap[f.VideoID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO favorites (user_id, video_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			uid, vid); err == nil {
			counts["favorites"]++
		}
	}
	for _, f := range backup.SeriesFavorites {
		uid, ok1 := userMap[f.UserID]
		sid, ok2 := seriesMap[f.SeriesID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO series_favorites (user_id, series_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			uid, sid); err == nil {
			counts["series_favorites"]++
		}
	}
	for _, p := range backup.WatchProgress {
		uid, ok1 := userMap[p.UserID]
		vid, ok2 := videoMap[p.VideoID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO watch_progress (user_id, video_id, position_sec, duration_sec)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id, video_id)
			DO UPDATE SET position_sec=EXCLUDED.position_sec, duration_sec=EXCLUDED.duration_sec,
			              updated_at=now()`,
			uid, vid, p.PositionSec, p.DurationSec); err == nil {
			counts["watch_progress"]++
		}
	}
	for _, h := range backup.HiddenPaths {
		uid, ok := userMap[h.UserID]
		if !ok || h.Path == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO hidden_paths (user_id, path) VALUES ($1,$2) ON CONFLICT (user_id, path) DO NOTHING`,
			uid, h.Path); err == nil {
			counts["hidden_paths"]++
		}
	}

	playlistMap := map[string]uuid.UUID{}
	for _, p := range backup.Playlists {
		uid, ok := userMap[p.UserID]
		if !ok {
			continue
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO playlists (user_id, name, description) VALUES ($1,$2,$3) RETURNING id`,
			uid, p.Name, p.Description).Scan(&id); err != nil {
			continue
		}
		playlistMap[p.ID] = id
		counts["playlists"]++
	}
	for _, pi := range backup.PlaylistItems {
		pid, ok1 := playlistMap[pi.PlaylistID]
		vid, ok2 := videoMap[pi.VideoID]
		if !ok1 || !ok2 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO playlist_items (playlist_id, video_id, position) VALUES ($1,$2,$3)
			ON CONFLICT (playlist_id, video_id) DO UPDATE SET position=EXCLUDED.position`,
			pid, vid, pi.Position); err == nil {
			counts["playlist_items"]++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "commit import failed: "+err.Error())
		return
	}
	counts["users_skipped"] = len(backup.Users) - len(userMap)
	writeJSON(w, http.StatusOK, map[string]any{"message": "backup imported", "counts": counts})
}
