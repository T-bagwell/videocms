package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"videocms/backend/internal/auth"
)

// GET /api/users/me/stats
func (a *App) userWatchStats(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFrom(r).ID
	var totalSec float64
	var plays, movies, episodes int
	if err := a.pool.QueryRow(r.Context(), `
		SELECT COALESCE(sum(wp.position_sec), 0)::float8,
		       count(*),
		       count(*) FILTER (WHERE v.series_id IS NULL),
		       count(*) FILTER (WHERE v.series_id IS NOT NULL)
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		WHERE wp.user_id=$1 AND wp.position_sec > 5`,
		userID).Scan(&totalSec, &plays, &movies, &episodes); err != nil {
		writeErr(w, http.StatusInternalServerError, "query watch stats failed")
		return
	}
	var daysActive int
	if err := a.pool.QueryRow(r.Context(), `
		SELECT count(DISTINCT wp.updated_at::date) FROM watch_progress wp
		WHERE wp.user_id=$1`, userID).Scan(&daysActive); err != nil {
		daysActive = 0
	}

	rows, err := a.pool.Query(r.Context(), `
		SELECT wp.updated_at::date, COALESCE(sum(wp.position_sec), 0)::float8
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		WHERE wp.user_id=$1
		GROUP BY wp.updated_at::date ORDER BY wp.updated_at::date DESC LIMIT 14`, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query weekly stats failed")
		return
	}
	weekly := []map[string]any{}
	for rows.Next() {
		var d time.Time
		var sec float64
		if err := rows.Scan(&d, &sec); err == nil {
			weekly = append(weekly, map[string]any{"date": d.Format("2006-01-02"), "seconds": sec})
		}
	}
	rows.Close()

	genreRows, err := a.pool.Query(r.Context(), `
		SELECT g, count(*) FROM (
			SELECT unnest(v.genres) AS g
			FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
			WHERE wp.user_id=$1 AND wp.position_sec > 5
		) t WHERE g <> '' GROUP BY g ORDER BY count(*) DESC LIMIT 6`, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query genre stats failed")
		return
	}
	defer genreRows.Close()
	genres := []map[string]any{}
	for genreRows.Next() {
		var g string
		var c int
		if err := genreRows.Scan(&g, &c); err == nil {
			genres = append(genres, map[string]any{"name": g, "count": c})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_watch_sec":  totalSec,
		"plays":            plays,
		"movies_watched":   movies,
		"episodes_watched": episodes,
		"days_active":      daysActive,
		"weekly":           weekly,
		"top_genres":       genres,
	})
}

// GET /api/admin/stats/watch
func (a *App) adminWatchStats(w http.ResponseWriter, r *http.Request) {
	var totalSec float64
	var activeUsers, plays int
	if err := a.pool.QueryRow(r.Context(), `
		SELECT COALESCE(sum(wp.position_sec), 0)::float8,
		       count(DISTINCT wp.user_id),
		       count(*)
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		WHERE wp.position_sec > 5`).Scan(&totalSec, &activeUsers, &plays); err != nil {
		writeErr(w, http.StatusInternalServerError, "query admin watch stats failed")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT v.title, COALESCE(sum(wp.position_sec), 0)::float8
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		GROUP BY v.id ORDER BY sum(wp.position_sec) DESC LIMIT 5`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query top videos failed")
		return
	}
	defer rows.Close()
	top := []map[string]any{}
	for rows.Next() {
		var title string
		var sec float64
		if err := rows.Scan(&title, &sec); err == nil {
			top = append(top, map[string]any{"title": title, "seconds": sec})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_watch_sec": totalSec,
		"active_users":    activeUsers,
		"total_plays":     plays,
		"top_videos":      top,
	})
}

// GET /api/users/me/stats/export
func (a *App) exportUserWatchStats(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFrom(r).ID
	rows, err := a.pool.Query(r.Context(), `
		SELECT v.title, wp.position_sec, wp.duration_sec, wp.updated_at
		FROM watch_progress wp JOIN videos v ON v.id = wp.video_id
		WHERE wp.user_id=$1 ORDER BY wp.updated_at DESC`, userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query watch history failed")
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="watch-stats.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"title", "position_sec", "duration_sec", "updated_at"})
	for rows.Next() {
		var title string
		var pos, dur float64
		var at time.Time
		if err := rows.Scan(&title, &pos, &dur, &at); err != nil {
			continue
		}
		_ = cw.Write([]string{title,
			strconv.FormatFloat(pos, 'f', 0, 64),
			strconv.FormatFloat(dur, 'f', 0, 64),
			at.Format(time.RFC3339)})
	}
	cw.Flush()
}

// GET /api/admin/stats/watch/export
func (a *App) exportAdminWatchStats(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT u.username, v.title, wp.position_sec, wp.updated_at
		FROM watch_progress wp
		JOIN users u ON u.id = wp.user_id
		JOIN videos v ON v.id = wp.video_id
		ORDER BY wp.updated_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query watch history failed")
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="watch-stats-all.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"username", "title", "position_sec", "updated_at"})
	for rows.Next() {
		var user, title string
		var pos float64
		var at time.Time
		if err := rows.Scan(&user, &title, &pos, &at); err != nil {
			continue
		}
		_ = cw.Write([]string{user, title, fmt.Sprintf("%.0f", pos), at.Format(time.RFC3339)})
	}
	cw.Flush()
}
