package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
	"videocms/backend/internal/models"
)

// buildBackup assembles the full JSON backup payload (same shape as the
// manual export endpoint).
func (a *App) buildBackup(ctx context.Context) (map[string]any, error) {
	out := map[string]any{"exported_at": time.Now().UTC()}
	var err error
	queries := map[string]string{
		"users":            `SELECT id, username, role, display_name, created_at FROM users ORDER BY username`,
		"libraries":        `SELECT id, name, path, blocked, video_count, created_at FROM libraries ORDER BY name`,
		"videos":           `SELECT id, library_id, title, filename, file_path, size_bytes, duration_sec, width, height, video_codec, container, year, synopsis, genres, tmdb_id, scraped_at, available, created_at, updated_at FROM videos ORDER BY title`,
		"series":           `SELECT id, library_id, name, season, episode_count, updated_at FROM series ORDER BY name, season`,
		"playlists":        `SELECT id, user_id, name, description, created_at, updated_at FROM playlists ORDER BY name`,
		"playlist_items":   `SELECT playlist_id, video_id, position FROM playlist_items ORDER BY playlist_id, position`,
		"favorites":        `SELECT user_id, video_id, created_at FROM favorites ORDER BY user_id`,
		"series_favorites": `SELECT user_id, series_id, created_at FROM series_favorites ORDER BY user_id`,
		"watch_progress":   `SELECT user_id, video_id, position_sec, duration_sec, updated_at FROM watch_progress ORDER BY user_id`,
		"hidden_paths":     `SELECT id, user_id, path, created_at FROM hidden_paths ORDER BY user_id`,
		"blocked_titles":   `SELECT id, title, created_at FROM blocked_titles ORDER BY title`,
	}
	for key, q := range queries {
		if out[key], err = a.queryRowsAny(ctx, q); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (a *App) backupsDir() string {
	return filepath.Join(a.cfg.DataDir, "backups")
}

// runMaintenance performs backup, health checks and (optionally) rescanning,
// returning a summary.
func (a *App) runMaintenance(ctx context.Context) (map[string]any, error) {
	if err := os.MkdirAll(a.backupsDir(), 0o755); err != nil {
		return nil, err
	}
	out, err := a.buildBackup(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	name := "videocms-backup-" + time.Now().Format("20060102-150405") + ".json"
	if err := os.WriteFile(filepath.Join(a.backupsDir(), name), raw, 0o644); err != nil {
		return nil, err
	}
	a.pruneBackups()

	summary := map[string]any{"backup": name}
	rows, err := a.pool.Query(ctx, `SELECT id, name FROM libraries ORDER BY name`)
	if err != nil {
		return summary, nil
	}
	defer rows.Close()
	healthTotal := 0
	healthIssues := 0
	var libs []struct {
		ID   string
		Name string
	}
	for rows.Next() {
		var l struct {
			ID   string
			Name string
		}
		if err := rows.Scan(&l.ID, &l.Name); err == nil {
			libs = append(libs, l)
		}
	}
	for _, l := range libs {
		id, _ := uuidParse(l.ID)
		if report, err := media.RunHealthCheck(ctx, a.pool, id); err == nil {
			healthTotal += report.Checked
			healthIssues += len(report.Missing) + len(report.Corrupt) + len(report.Duplicates)
		}
	}
	summary["health"] = map[string]any{"checked": healthTotal, "issues": healthIssues}
	if a.cfg.MaintRescan {
		rescanned := 0
		for _, l := range libs {
			id, _ := uuidParse(l.ID)
			var lib models.Library
			if err := a.pool.QueryRow(ctx,
				`SELECT id, name, path, scan_status, scan_error, scan_started_at, scan_finished_at, video_count, blocked, created_at, quota_bytes
				 FROM libraries WHERE id=$1`, id).Scan(&lib.ID, &lib.Name, &lib.Path, &lib.ScanStatus,
				&lib.ScanError, &lib.ScanStartedAt, &lib.ScanFinishedAt, &lib.VideoCount, &lib.Blocked, &lib.CreatedAt, &lib.QuotaBytes); err == nil {
				if a.scanner.Start(ctx, lib) {
					rescanned++
				}
			}
		}
		summary["rescanned"] = rescanned
	}
	return summary, nil
}

func (a *App) pruneBackups() {
	retention := a.cfg.MaintRetention
	if retention <= 0 {
		retention = 7
	}
	entries, err := os.ReadDir(a.backupsDir())
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "videocms-backup-") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i, n := range names {
		if i >= retention {
			_ = os.Remove(filepath.Join(a.backupsDir(), n))
		}
	}
}

// StartMaintenance runs the scheduled maintenance loop until ctx is done.
func (a *App) StartMaintenance(ctx context.Context) {
	hours := a.cfg.MaintIntervalHours
	if hours <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(hours) * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := a.runMaintenance(ctx); err != nil {
					log.Printf("maintenance: %v", err)
				}
			}
		}
	}()
}

// POST /api/admin/maintenance/run — run maintenance now.
func (a *App) runMaintenanceNow(w http.ResponseWriter, r *http.Request) {
	summary, err := a.runMaintenance(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "maintenance failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// GET /api/admin/backups — list backup files.
func (a *App) listBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(a.backupsDir())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	items := []map[string]any{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "videocms-backup-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, map[string]any{
			"name": e.Name(), "size": info.Size(),
			"created_at": info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["name"].(string) > items[j]["name"].(string) })
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/admin/backups/{name} — download a backup file.
func (a *App) downloadBackup(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	if !strings.HasPrefix(name, "videocms-backup-") || !strings.HasSuffix(name, ".json") {
		writeErr(w, http.StatusBadRequest, "invalid backup name")
		return
	}
	path := filepath.Join(a.backupsDir(), name)
	if !strings.HasPrefix(path, a.backupsDir()+string(os.PathSeparator)) {
		writeErr(w, http.StatusBadRequest, "invalid backup name")
		return
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		writeErr(w, http.StatusNotFound, "backup not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, path)
}

func uuidParse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
