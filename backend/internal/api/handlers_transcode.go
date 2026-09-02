package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

type transcodeJob struct {
	ID         uuid.UUID  `json:"id"`
	VideoID    uuid.UUID  `json:"video_id"`
	VideoTitle string     `json:"video_title"`
	Priority   int        `json:"priority"`
	Status     string     `json:"status"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// GET /api/admin/transcode/jobs
func (a *App) listTranscodeJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT tj.id, tj.video_id, COALESCE(v.title, ''), tj.priority, tj.status,
		       tj.error, tj.created_at, tj.finished_at
		FROM transcode_jobs tj LEFT JOIN videos v ON v.id = tj.video_id
		ORDER BY (tj.status='queued' OR tj.status='running') DESC, tj.created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query transcode jobs failed")
		return
	}
	defer rows.Close()
	items := []transcodeJob{}
	for rows.Next() {
		var j transcodeJob
		if err := rows.Scan(&j.ID, &j.VideoID, &j.VideoTitle, &j.Priority, &j.Status,
			&j.Error, &j.CreatedAt, &j.FinishedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan transcode job failed")
			return
		}
		items = append(items, j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "workers": a.cfg.TranscodeWorkers})
}

// POST /api/admin/transcode/queue
func (a *App) queueTranscode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VideoID  string `json:"video_id"`
		Priority int    `json:"priority"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	videoID, err := uuid.Parse(req.VideoID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video_id")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM videos WHERE id=$1)`, videoID).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	var j transcodeJob
	err = a.pool.QueryRow(r.Context(), `
		INSERT INTO transcode_jobs (video_id, priority)
		VALUES ($1,$2) RETURNING id, video_id, '', priority, status, '', created_at, NULL`,
		videoID, req.Priority).Scan(&j.ID, &j.VideoID, &j.VideoTitle, &j.Priority,
		&j.Status, &j.Error, &j.CreatedAt, &j.FinishedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "queue transcode failed")
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

// DELETE /api/admin/transcode/jobs/{id}
func (a *App) cancelTranscode(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		UPDATE transcode_jobs SET status='failed', error='cancelled', finished_at=now()
		WHERE id=$1 AND status IN ('queued','running')`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "cancel transcode failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "cancelled"})
}
