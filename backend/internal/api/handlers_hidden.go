package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type hiddenPath struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (a *App) listHiddenPaths(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(),
		`SELECT id::text, path FROM hidden_paths WHERE user_id=$1 ORDER BY created_at DESC`,
		user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query hidden paths failed")
		return
	}
	defer rows.Close()

	items := []hiddenPath{}
	for rows.Next() {
		var p hiddenPath
		if err := rows.Scan(&p.ID, &p.Path); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan hidden path failed")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) addHiddenPath(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	if !filepath.IsAbs(path) {
		writeErr(w, http.StatusBadRequest, "path must be an absolute server path")
		return
	}
	// normalize trailing separators so "/a/b/" and "/a/b" match the same subtree
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}

	var id string
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO hidden_paths (user_id, path) VALUES ($1,$2)
		ON CONFLICT (user_id, path) DO NOTHING
		RETURNING id::text`, user.ID, path).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSON(w, http.StatusOK, map[string]any{"message": "already hidden"})
			return
		}
		writeErr(w, http.StatusInternalServerError, "add hidden path failed")
		return
	}
	writeJSON(w, http.StatusCreated, hiddenPath{ID: id, Path: path})
}

func (a *App) removeHiddenPath(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid hidden path id")
		return
	}
	user := auth.UserFrom(r)
	tag, err := a.pool.Exec(r.Context(),
		`DELETE FROM hidden_paths WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "remove hidden path failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "hidden path not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}
