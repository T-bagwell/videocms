package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type scraperReg struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Command   string    `json:"command,omitempty"`
	URL       string    `json:"url,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// GET /api/admin/scrapers
func (a *App) listScrapers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, kind, command, url, enabled, created_at
		FROM scrapers ORDER BY lower(name)`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query scrapers failed")
		return
	}
	defer rows.Close()
	items := []scraperReg{}
	for rows.Next() {
		var s scraperReg
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.Command, &s.URL, &s.Enabled, &s.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan scraper failed")
			return
		}
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/scrapers registers an external scraper.
func (a *App) registerScraper(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Command string `json:"command"`
		URL     string `json:"url"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Kind != "command" && req.Kind != "url" {
		writeErr(w, http.StatusBadRequest, "kind must be command or url")
		return
	}
	if req.Kind == "command" && strings.TrimSpace(req.Command) == "" {
		writeErr(w, http.StatusBadRequest, "command is required for command scrapers")
		return
	}
	if req.Kind == "url" && strings.TrimSpace(req.URL) == "" {
		writeErr(w, http.StatusBadRequest, "url is required for url scrapers")
		return
	}
	var s scraperReg
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO scrapers (name, kind, command, url)
		VALUES ($1,$2,$3,$4) RETURNING id, name, kind, command, url, enabled, created_at`,
		req.Name, req.Kind, strings.TrimSpace(req.Command), strings.TrimSpace(req.URL)).
		Scan(&s.ID, &s.Name, &s.Kind, &s.Command, &s.URL, &s.Enabled, &s.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusConflict, "scraper name already exists")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

// PATCH /api/admin/scrapers/{id}
func (a *App) updateScraper(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid scraper id")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Enabled == nil {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE scrapers SET enabled=$1 WHERE id=$2`, *req.Enabled, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update scraper failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

// DELETE /api/admin/scrapers/{id}
func (a *App) deleteScraper(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid scraper id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM scrapers WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete scraper failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}
