package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultYtDlpFormat = "bv*+ba/b"

type downloadJob struct {
	ID           uuid.UUID  `json:"id"`
	URL          string     `json:"url"`
	Title        string     `json:"title"`
	TargetPath   string     `json:"target_path"`
	Format       string     `json:"format"`
	Status       string     `json:"status"`
	Progress     float64    `json:"progress"`
	Error        string     `json:"error,omitempty"`
	IntervalSecs int64      `json:"interval_secs"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type createDownloadRequest struct {
	URL          string `json:"url"`
	TargetPath   string `json:"target_path"`
	Format       string `json:"format"`
	IntervalSecs int64  `json:"interval_secs"`
}

const downloadColumns = `id, url, title, target_path, format, status, progress, error,
	interval_secs, last_run_at, created_at`

func scanDownload(rows interface{ Scan(...any) error }) (downloadJob, error) {
	var j downloadJob
	err := rows.Scan(&j.ID, &j.URL, &j.Title, &j.TargetPath, &j.Format, &j.Status,
		&j.Progress, &j.Error, &j.IntervalSecs, &j.LastRunAt, &j.CreatedAt)
	return j, err
}

// GET /api/downloads — admin list of yt-dlp download jobs.
func (a *App) listDownloads(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT `+downloadColumns+`
		FROM downloads ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list downloads failed")
		return
	}
	defer rows.Close()

	items := []downloadJob{}
	for rows.Next() {
		j, err := scanDownload(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan download failed")
			return
		}
		items = append(items, j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/downloads — queue a yt-dlp job. interval_secs > 0 re-runs the job
// on that schedule after the first successful download.
func (a *App) createDownload(w http.ResponseWriter, r *http.Request) {
	var req createDownloadRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		writeErr(w, http.StatusBadRequest, "url must start with http:// or https://")
		return
	}
	if !isAbsDir(req.TargetPath) {
		if resolved, ok := a.resolveTargetPath(req.TargetPath); !ok {
			writeErr(w, http.StatusBadRequest, "target_path must be an existing absolute directory or pool reference")
			return
		} else if !isAbsDir(resolved) {
			writeErr(w, http.StatusBadRequest, "target_path must be an existing absolute directory or pool reference")
			return
		} else {
			req.TargetPath = resolved
		}
	}
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = defaultYtDlpFormat
	}
	if req.IntervalSecs < 0 {
		req.IntervalSecs = 0
	}

	var j downloadJob
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO downloads (url, target_path, format, interval_secs)
		VALUES ($1, $2, $3, $4)
		RETURNING `+downloadColumns,
		req.URL, req.TargetPath, format, req.IntervalSecs).Scan(
		&j.ID, &j.URL, &j.Title, &j.TargetPath, &j.Format, &j.Status,
		&j.Progress, &j.Error, &j.IntervalSecs, &j.LastRunAt, &j.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create download failed")
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

// DELETE /api/downloads/{id} — cancel a queued or running job.
func (a *App) deleteDownload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid download id")
		return
	}
	// Mark the job canceled first so the worker's post-start re-check always
	// sees the canceled status; only then kill the running process.
	tag, err := a.pool.Exec(r.Context(), `
		UPDATE downloads SET status='canceled', updated_at=now()
		WHERE id=$1 AND status IN ('queued', 'downloading')`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cancel download failed")
		return
	}
	if tag.RowsAffected() == 1 {
		a.dl.Cancel(id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"canceled": true})
}

// POST /api/downloads/{id}/retry — requeue a failed or canceled job.
func (a *App) retryDownload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid download id")
		return
	}
	tag, err := a.pool.Exec(r.Context(), `
		UPDATE downloads SET status='queued', progress=0, error='', updated_at=now()
		WHERE id=$1 AND status IN ('failed', 'canceled')`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "retry download failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusConflict, "download is not retryable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": true})
}
