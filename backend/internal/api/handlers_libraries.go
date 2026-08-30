package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/models"
)

func (a *App) listLibraries(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT l.id, l.name, l.path, l.scan_status, l.scan_error, l.scan_started_at,
		       l.scan_finished_at, l.video_count, l.blocked, l.created_at, l.quota_bytes
		FROM libraries l
		ORDER BY l.created_at DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query libraries failed")
		return
	}
	defer rows.Close()

	libs := []models.Library{}
	for rows.Next() {
		var lib models.Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.Path, &lib.ScanStatus, &lib.ScanError,
			&lib.ScanStartedAt, &lib.ScanFinishedAt, &lib.VideoCount, &lib.Blocked, &lib.CreatedAt, &lib.QuotaBytes); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan library row failed")
			return
		}
		libs = append(libs, lib)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": libs})
}

type createLibraryRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *App) createLibrary(w http.ResponseWriter, r *http.Request) {
	var req createLibraryRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if req.Name == "" || req.Path == "" {
		writeErr(w, http.StatusBadRequest, "name and path are required")
		return
	}
	if !filepath.IsAbs(req.Path) {
		writeErr(w, http.StatusBadRequest, "path must be an absolute server path")
		return
	}
	abs := filepath.Clean("/" + req.Path)
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		writeErr(w, http.StatusBadRequest, "path does not exist or is not a directory: "+abs)
		return
	}

	lib := models.Library{Name: req.Name, Path: abs}
	err = a.pool.QueryRow(r.Context(),
		`INSERT INTO libraries (name, path) VALUES ($1,$2) RETURNING id, created_at`,
		req.Name, abs,
	).Scan(&lib.ID, &lib.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeErr(w, http.StatusConflict, "a library for this path already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, "create library failed")
		return
	}
	writeJSON(w, http.StatusCreated, lib)
}

func (a *App) scanLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	var lib models.Library
	err = a.pool.QueryRow(r.Context(),
		`SELECT id, name, path, scan_status, scan_error, scan_started_at, scan_finished_at, video_count, created_at
		 FROM libraries WHERE id=$1`, id,
	).Scan(&lib.ID, &lib.Name, &lib.Path, &lib.ScanStatus, &lib.ScanError,
		&lib.ScanStartedAt, &lib.ScanFinishedAt, &lib.VideoCount, &lib.CreatedAt)
	if err == pgx.ErrNoRows {
		writeErr(w, http.StatusNotFound, "library not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load library failed")
		return
	}
	if !a.scanner.Start(context.Background(), lib) {
		writeErr(w, http.StatusConflict, "library is already scanning")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": "scan started", "library_id": id})
}

func (a *App) cancelScan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	if !a.scanner.Cancel(id) {
		writeErr(w, http.StatusConflict, "library has no running scan")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": "scan cancelled"})
}

func (a *App) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	tag, err := a.pool.Exec(r.Context(), `DELETE FROM libraries WHERE id=$1`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete library failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "library not found")
		return
	}
	// best-effort cleanup of generated posters for this library's videos
	go func() {
		rows, err := a.pool.Query(r.Context(),
			`SELECT poster_path FROM videos WHERE library_id=$1`, id)
		if err != nil {
			return
		}
		defer rows.Close()
		var paths []string
		for rows.Next() {
			var p string
			_ = rows.Scan(&p)
			if p != "" {
				paths = append(paths, p)
			}
		}
		for _, p := range paths {
			if filepath.Dir(p) == filepath.Join(a.cfg.DataDir, "posters") {
				_ = os.Remove(p)
			}
		}
	}()
	writeJSON(w, http.StatusOK, map[string]any{"message": "library deleted"})
}

// openLibrary opens the library folder on the server with the system file
// manager (Finder / Files / Nautilus…). This runs on the server machine, not
// on the browser client.
func (a *App) openLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(),
		`SELECT path FROM libraries WHERE id=$1`, id).Scan(&path)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "library not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "query library failed")
		return
	}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		writeErr(w, http.StatusNotFound, "library folder does not exist on the server")
		return
	}

	var opener string
	switch runtime.GOOS {
	case "darwin":
		opener = "open"
	case "windows":
		opener = "explorer"
	default:
		opener = "xdg-open"
	}
	cmd := exec.Command(opener, path)
	if err := cmd.Start(); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to open folder: "+err.Error())
		return
	}
	// release resources without blocking the HTTP response
	go func() { _ = cmd.Wait() }()
	writeJSON(w, http.StatusOK, map[string]any{"message": "opened", "path": path})
}

// setLibraryBlocked toggles a library-wide content block. Blocked libraries
// disappear from every user-facing listing; files and records are kept.
func (a *App) setLibraryBlocked(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	var req struct {
		Blocked    bool   `json:"blocked"`
		QuotaBytes *int64 `json:"quota_bytes"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	tag, err := a.pool.Exec(r.Context(),
		`UPDATE libraries SET blocked=$2, quota_bytes=COALESCE($3, quota_bytes) WHERE id=$1`,
		id, req.Blocked, req.QuotaBytes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "update library failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "library not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated", "blocked": req.Blocked})
}

func isUniqueViolation(err error) bool {
	type sqlState interface{ SQLState() string }
	if se, ok := err.(sqlState); ok {
		return se.SQLState() == "23505"
	}
	return false
}
