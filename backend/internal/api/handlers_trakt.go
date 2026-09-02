package api

import (
	"net/http"
	"time"

	"videocms/backend/internal/media"
)

// GET /api/admin/trakt/status
func (a *App) traktStatus(w http.ResponseWriter, r *http.Request) {
	var lastSync map[string]any
	var syncedAt *time.Time
	var count int
	var syncErr string
	if err := a.pool.QueryRow(r.Context(), `
		SELECT synced_at, item_count, error FROM trakt_sync_log
		ORDER BY synced_at DESC LIMIT 1`).Scan(&syncedAt, &count, &syncErr); err == nil && syncedAt != nil {
		lastSync = map[string]any{"synced_at": syncedAt, "item_count": count, "error": syncErr}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":    a.cfg.TraktClientID != "" && a.cfg.TraktAccessToken != "",
		"client_id":     a.cfg.TraktClientID != "",
		"access_token":  a.cfg.TraktAccessToken != "",
		"refresh_token": a.cfg.TraktRefreshToken != "",
		"last_sync":     lastSync,
	})
}

// POST /api/admin/trakt/sync pushes watch history to Trakt.
func (a *App) traktSync(w http.ResponseWriter, r *http.Request) {
	if a.cfg.TraktClientID == "" || a.cfg.TraktAccessToken == "" {
		writeErr(w, http.StatusBadRequest,
			"trakt not configured (set TRAKT_CLIENT_ID and TRAKT_ACCESS_TOKEN)")
		return
	}
	client := media.NewTraktClient(a.cfg.TraktClientID, a.cfg.TraktAccessToken, a.cfg.TraktRefreshToken)
	if a.cfg.TraktRefreshToken != "" {
		if err := client.Refresh(r.Context()); err != nil {
			writeErr(w, http.StatusBadGateway, "trakt token refresh failed: "+err.Error())
			return
		}
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT DISTINCT ON (v.tmdb_id) v.tmdb_id, wp.updated_at
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		WHERE v.tmdb_id > 0 AND wp.position_sec >= 30
		ORDER BY v.tmdb_id, wp.updated_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query watch history failed")
		return
	}
	var items []media.TraktItem
	for rows.Next() {
		var id int
		var at time.Time
		if err := rows.Scan(&id, &at); err == nil {
			items = append(items, media.TraktItem{TmdbID: id, WatchedAt: at})
		}
	}
	rows.Close()

	pushed, err := client.SyncHistory(r.Context(), items)
	if err != nil {
		_, _ = a.pool.Exec(r.Context(),
			`INSERT INTO trakt_sync_log (item_count, error) VALUES (0, $1)`, err.Error())
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`INSERT INTO trakt_sync_log (item_count, error) VALUES ($1, '')`, pushed); err != nil {
		writeErr(w, http.StatusInternalServerError, "record sync log failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pushed": pushed})
}
