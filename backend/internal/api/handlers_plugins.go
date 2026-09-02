package api

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type pluginReg struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	InstallURL  string    `json:"install_url,omitempty"`
	Kind        string    `json:"kind"`
	Events      []string  `json:"events"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// pluginDirectory is the built-in community directory of installable plugins.
var pluginDirectory = []map[string]string{
	{"name": "example-notify", "description": "Posts server events to an example webhook endpoint", "install_url": "https://example.com/hooks/videocms", "kind": "webhook"},
	{"name": "event-logger", "description": "Logs server events locally via a command", "kind": "command"},
}

// GET /api/plugins/directory
func (a *App) pluginDirectory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": pluginDirectory})
}

// GET /api/admin/plugins
func (a *App) listPlugins(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, description, install_url, kind, events, enabled, created_at
		FROM plugins ORDER BY lower(name)`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query plugins failed")
		return
	}
	defer rows.Close()
	items := []pluginReg{}
	for rows.Next() {
		var p pluginReg
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.InstallURL, &p.Kind,
			&p.Events, &p.Enabled, &p.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan plugin failed")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/plugins installs a plugin.
func (a *App) installPlugin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		InstallURL  string   `json:"install_url"`
		Kind        string   `json:"kind"`
		Events      []string `json:"events"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Kind == "" {
		req.Kind = "webhook"
	}
	if req.Kind != "webhook" && req.Kind != "command" {
		writeErr(w, http.StatusBadRequest, "kind must be webhook or command")
		return
	}
	var p pluginReg
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO plugins (name, description, install_url, kind, events)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, name, description, install_url, kind, events, enabled, created_at`,
		req.Name, strings.TrimSpace(req.Description), strings.TrimSpace(req.InstallURL),
		req.Kind, req.Events).Scan(&p.ID, &p.Name, &p.Description, &p.InstallURL,
		&p.Kind, &p.Events, &p.Enabled, &p.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusConflict, "plugin name already exists")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// PATCH /api/admin/plugins/{id}
func (a *App) updatePlugin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid plugin id")
		return
	}
	var req struct {
		Enabled *bool    `json:"enabled"`
		Events  []string `json:"events"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Enabled == nil && req.Events == nil {
		writeErr(w, http.StatusBadRequest, "nothing to update")
		return
	}
	if req.Enabled != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE plugins SET enabled=$1 WHERE id=$2`, *req.Enabled, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update plugin failed")
			return
		}
	}
	if req.Events != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE plugins SET events=$1 WHERE id=$2`, req.Events, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "update plugin failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

// DELETE /api/admin/plugins/{id}
func (a *App) uninstallPlugin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid plugin id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM plugins WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete plugin failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// deliverPluginEvents posts matching webhook plugins for an event.
func (a *App) deliverPluginEvents(raw []byte, event string) {
	rows, err := a.pool.Query(context.Background(), `
		SELECT name, install_url FROM plugins
		WHERE kind='webhook' AND enabled=true AND install_url <> '' AND events @> ARRAY[$1]::text[]`, event)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name, url string
		if err := rows.Scan(&name, &url); err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url,
			bytes.NewReader(raw))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Videocms-Plugin", name)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}
