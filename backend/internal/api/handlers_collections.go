package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type collection struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Query     json.RawMessage `json:"query"`
	CreatedAt time.Time       `json:"created_at"`
}

type collectionRequest struct {
	Name  string          `json:"name"`
	Query json.RawMessage `json:"query"`
}

// GET /api/collections — the current user's smart collections.
func (a *App) listCollections(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, query, created_at FROM collections
		WHERE user_id=$1 ORDER BY created_at`, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list collections failed")
		return
	}
	defer rows.Close()
	items := []collection{}
	for rows.Next() {
		var c collection
		if err := rows.Scan(&c.ID, &c.Name, &c.Query, &c.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan collection failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/collections — save the current filter set as a smart collection.
func (a *App) createCollection(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req collectionRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		writeErr(w, http.StatusBadRequest, "name must be 1-100 characters")
		return
	}
	if len(req.Query) == 0 {
		req.Query = json.RawMessage(`{}`)
	}
	var c collection
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO collections (user_id, name, query)
		VALUES ($1, $2, $3)
		RETURNING id, name, query, created_at`,
		user.ID, req.Name, req.Query).Scan(&c.ID, &c.Name, &c.Query, &c.CreatedAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "create collection failed")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// DELETE /api/collections/{id}
func (a *App) deleteCollection(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid collection id")
		return
	}
	tag, err := a.pool.Exec(r.Context(),
		`DELETE FROM collections WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete collection failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "collection not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// GET /api/users/me/filters — the user's saved browse filters.
func (a *App) getUserFilters(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var raw json.RawMessage
	err := a.pool.QueryRow(r.Context(),
		`SELECT filters FROM user_filter_prefs WHERE user_id=$1`, user.ID).Scan(&raw)
	if err != nil {
		raw = json.RawMessage(`{}`)
	}
	writeJSON(w, http.StatusOK, map[string]any{"filters": json.RawMessage(raw)})
}

// PUT /api/users/me/filters — save the current browse filters.
func (a *App) saveUserFilters(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req struct {
		Filters json.RawMessage `json:"filters"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Filters) == 0 {
		req.Filters = json.RawMessage(`{}`)
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO user_filter_prefs (user_id, filters)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET filters=EXCLUDED.filters, updated_at=now()`,
		user.ID, req.Filters); err != nil {
		writeErr(w, http.StatusInternalServerError, "save filters failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"filters": req.Filters})
}
