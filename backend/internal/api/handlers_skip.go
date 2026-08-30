package api

import (
	"net/http"

	"github.com/google/uuid"
)

type skipInterval struct {
	Start float64 `json:"start_sec"`
	End   float64 `json:"end_sec"`
}

// GET /api/videos/{id}/skip-intervals
func (a *App) getSkipIntervals(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	out := map[string]any{"intro": nil, "credits": nil}
	rows, err := a.pool.Query(r.Context(), `
		SELECT kind, start_sec, end_sec FROM skip_intervals WHERE video_id=$1`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load skip intervals failed")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var s skipInterval
		if err := rows.Scan(&kind, &s.Start, &s.End); err == nil {
			out[kind] = s
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /api/videos/{id}/skip-interval
func (a *App) setSkipInterval(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req struct {
		Kind string  `json:"kind"`
		From float64 `json:"start_sec"`
		To   float64 `json:"end_sec"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Kind != "intro" && req.Kind != "credits" {
		writeErr(w, http.StatusBadRequest, "kind must be intro or credits")
		return
	}
	if req.From < 0 || req.To <= req.From {
		writeErr(w, http.StatusBadRequest, "invalid interval")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO skip_intervals (video_id, kind, start_sec, end_sec)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (video_id, kind)
		DO UPDATE SET start_sec=EXCLUDED.start_sec, end_sec=EXCLUDED.end_sec, updated_at=now()`,
		id, req.Kind, req.From, req.To); err != nil {
		writeErr(w, http.StatusBadRequest, "video not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": req.Kind, "start_sec": req.From, "end_sec": req.To})
}

// DELETE /api/videos/{id}/skip-interval?kind=intro
func (a *App) clearSkipInterval(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "intro" && kind != "credits" {
		writeErr(w, http.StatusBadRequest, "kind must be intro or credits")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM skip_intervals WHERE video_id=$1 AND kind=$2`, id, kind); err != nil {
		writeErr(w, http.StatusInternalServerError, "clear skip interval failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cleared": true})
}
