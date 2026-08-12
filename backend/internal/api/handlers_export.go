package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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
			row[c.Name] = vals[i]
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func serveJSONDownload(w http.ResponseWriter, name string, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
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
