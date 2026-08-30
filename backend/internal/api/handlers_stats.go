package api

import (
	"net/http"
	"syscall"

	"videocms/backend/internal/models"
)

func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	var s models.Stats
	if err := a.pool.QueryRow(r.Context(), `
		SELECT
			(SELECT count(*) FROM users),
			(SELECT count(*) FROM libraries),
			(SELECT count(*) FROM series),
			(SELECT count(*) FROM videos),
			(SELECT count(*) FROM videos WHERE available=false),
			(SELECT count(*) FROM playlists),
			(SELECT count(*) FROM favorites),
			(SELECT COALESCE(sum(size_bytes),0) FROM videos)`).Scan(
		&s.Users, &s.Libraries, &s.Series, &s.Videos, &s.VideosMissing, &s.Playlists, &s.Favorites, &s.TotalBytes); err != nil {
		writeErr(w, http.StatusInternalServerError, "query stats failed")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type jobItem struct {
	Kind     string  `json:"kind"`
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error,omitempty"`
}

// GET /api/admin/jobs — unified view of scans, uploads, downloads and live
// streams for the admin dashboard.
func (a *App) jobs(w http.ResponseWriter, r *http.Request) {
	items := []jobItem{}
	rows, err := a.pool.Query(r.Context(), `
		SELECT id::text, name, scan_status, 0, scan_error FROM libraries ORDER BY created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var j jobItem
			if err := rows.Scan(&j.ID, &j.Title, &j.Status, &j.Progress, &j.Error); err == nil {
				j.Kind = "scan"
				items = append(items, j)
			}
		}
		rows.Close()
	}
	rows, err = a.pool.Query(r.Context(), `
		SELECT id::text, filename, status, 0, error FROM uploads ORDER BY created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var j jobItem
			if err := rows.Scan(&j.ID, &j.Title, &j.Status, &j.Progress, &j.Error); err == nil {
				j.Kind = "upload"
				items = append(items, j)
			}
		}
		rows.Close()
	}
	rows, err = a.pool.Query(r.Context(), `
		SELECT id::text, url, status, progress, error FROM downloads ORDER BY created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var j jobItem
			if err := rows.Scan(&j.ID, &j.Title, &j.Status, &j.Progress, &j.Error); err == nil {
				j.Kind = "download"
				items = append(items, j)
			}
		}
		rows.Close()
	}
	rows, err = a.pool.Query(r.Context(), `
		SELECT id::text, title, status, 0, error FROM live_streams ORDER BY created_at DESC LIMIT 100`)
	if err == nil {
		for rows.Next() {
			var j jobItem
			if err := rows.Scan(&j.ID, &j.Title, &j.Status, &j.Progress, &j.Error); err == nil {
				j.Kind = "live"
				items = append(items, j)
			}
		}
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// diskUsage returns free/total bytes for the data directory.
func diskUsage(path string) (free, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}

// GET /api/admin/system — disk stats for the dashboard.
func (a *App) system(w http.ResponseWriter, r *http.Request) {
	free, total := diskUsage(a.cfg.DataDir)
	writeJSON(w, http.StatusOK, map[string]any{"disk_free": free, "disk_total": total})
}
