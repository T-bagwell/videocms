package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type recording struct {
	ID        uuid.UUID `json:"id"`
	ChannelID uuid.UUID `json:"channel_id"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	StartUTC  time.Time `json:"start_utc"`
	EndUTC    time.Time `json:"end_utc"`
	Status    string    `json:"status"`
	FilePath  string    `json:"-"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GET /api/admin/recordings
func (a *App) listRecordings(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT rec.id, rec.channel_id, COALESCE(c.name, ''), rec.title,
		       rec.start_utc, rec.end_utc, rec.status, rec.file_path, rec.error, rec.created_at
		FROM recordings rec LEFT JOIN iptv_channels c ON c.id = rec.channel_id
		ORDER BY rec.start_utc DESC`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query recordings failed")
		return
	}
	defer rows.Close()
	items := []recording{}
	for rows.Next() {
		var rec recording
		if err := rows.Scan(&rec.ID, &rec.ChannelID, &rec.Channel, &rec.Title,
			&rec.StartUTC, &rec.EndUTC, &rec.Status, &rec.FilePath, &rec.Error, &rec.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan recording failed")
			return
		}
		items = append(items, rec)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/recordings schedules a recording.
func (a *App) createRecording(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channel_id"`
		Title     string `json:"title"`
		StartUTC  string `json:"start_utc"`
		EndUTC    string `json:"end_utc"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	channelID, err := uuid.Parse(req.ChannelID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid channel_id")
		return
	}
	start, err1 := time.Parse(time.RFC3339, req.StartUTC)
	end, err2 := time.Parse(time.RFC3339, req.EndUTC)
	if err1 != nil || err2 != nil {
		writeErr(w, http.StatusBadRequest, "start_utc/end_utc must be RFC3339 timestamps")
		return
	}
	if !end.After(start) {
		writeErr(w, http.StatusBadRequest, "end_utc must be after start_utc")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM iptv_channels WHERE id=$1)`, channelID).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "channel not found")
		return
	}
	var rec recording
	err = a.pool.QueryRow(r.Context(), `
		INSERT INTO recordings (channel_id, title, start_utc, end_utc)
		VALUES ($1,$2,$3,$4) RETURNING id, channel_id, '', title, start_utc, end_utc, status, '', '', created_at`,
		channelID, strings.TrimSpace(req.Title), start.UTC(), end.UTC()).
		Scan(&rec.ID, &rec.ChannelID, &rec.Channel, &rec.Title, &rec.StartUTC,
			&rec.EndUTC, &rec.Status, &rec.FilePath, &rec.Error, &rec.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create recording failed")
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

// DELETE /api/admin/recordings/{id}
func (a *App) deleteRecording(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid recording id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM recordings WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete recording failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// GET /api/admin/tuners lists configured HDHomeRun devices.
func (a *App) listTuners(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.cfg.HDHomeRunURLs})
}

// POST /api/admin/tuners/scan pulls each HDHomeRun lineup into channels.
func (a *App) scanTuners(w http.ResponseWriter, r *http.Request) {
	created := 0
	failed := 0
	for _, base := range a.cfg.HDHomeRunURLs {
		base = strings.TrimRight(base, "/")
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/lineup.json", nil)
		if err != nil {
			failed++
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			failed++
			continue
		}
		var lineup []struct {
			GuideNumber string `json:"GuideNumber"`
			GuideName   string `json:"GuideName"`
			URL         string `json:"URL"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&lineup)
		_ = resp.Body.Close()
		if err != nil {
			failed++
			continue
		}
		for _, ch := range lineup {
			name := strings.TrimSpace(ch.GuideName)
			if name == "" {
				name = ch.GuideNumber
			}
			if name == "" || strings.TrimSpace(ch.URL) == "" {
				continue
			}
			if _, err := a.pool.Exec(r.Context(), `
				INSERT INTO iptv_channels (name, tvg_id, tvg_name, group_title, source_url)
				VALUES ($1,$2,$3,'Tuner',$4)
				ON CONFLICT (source_url) WHERE source_url <> '' DO UPDATE SET
					name=EXCLUDED.name, tvg_id=EXCLUDED.tvg_id, tvg_name=EXCLUDED.tvg_name,
					group_title='Tuner', updated_at=now()`,
				name, ch.GuideNumber, ch.GuideName, ch.URL); err == nil {
				created++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": created, "failed": failed})
}
