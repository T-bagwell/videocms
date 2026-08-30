package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

type tagItem struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Kind string    `json:"kind"`
}

type addTagRequest struct {
	Name string `json:"name"`
}

// ensureTag returns the id of a tag with the given name, creating it if needed.
func (a *App) ensureTag(r *http.Request, name, kind string) (uuid.UUID, error) {
	var id uuid.UUID
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO tags (name, kind) VALUES ($1, $2)
		ON CONFLICT (name) DO NOTHING
		RETURNING id`, name, kind).Scan(&id)
	if err != nil {
		// tag already exists
		err = a.pool.QueryRow(r.Context(),
			`SELECT id FROM tags WHERE name=$1`, name).Scan(&id)
	}
	return id, err
}

// GET /api/videos/{id}/tags
func (a *App) listVideoTags(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT t.id, t.name, t.kind FROM tags t
		JOIN video_tags vt ON vt.tag_id = t.id
		WHERE vt.video_id=$1 ORDER BY t.name`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list tags failed")
		return
	}
	defer rows.Close()
	items := []tagItem{}
	for rows.Next() {
		var t tagItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Kind); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan tag failed")
			return
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/videos/{id}/tags — add a manual tag.
func (a *App) addVideoTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req addTagRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "" || len(req.Name) > 50 {
		writeErr(w, http.StatusBadRequest, "tag must be 1-50 characters")
		return
	}
	tagID, err := a.ensureTag(r, req.Name, "manual")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create tag failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO video_tags (video_id, tag_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, id, tagID); err != nil {
		writeErr(w, http.StatusInternalServerError, "add tag failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": tagID, "name": req.Name, "kind": "manual"})
}

// DELETE /api/videos/{id}/tags/{tagId}
func (a *App) removeVideoTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	tagID, err := uuid.Parse(r.PathValue("tagId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid tag id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM video_tags WHERE video_id=$1 AND tag_id=$2`, id, tagID); err != nil {
		writeErr(w, http.StatusInternalServerError, "remove tag failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": true})
}

// POST /api/videos/{id}/analyze — run the configured AI tagger (admin).
func (a *App) analyzeVideo(w http.ResponseWriter, r *http.Request) {
	if a.cfg.AITagBin == "" {
		writeErr(w, http.StatusBadRequest, "ai tagging is not configured (set AI_TAG_BIN)")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var path string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM videos WHERE id=$1 AND available`, id).Scan(&path); err != nil {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	tags, err := media.AnalyzeTags(r.Context(), a.cfg.AITagBin, path)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, name := range tags {
		tagID, err := a.ensureTag(r, name, "auto")
		if err != nil {
			continue
		}
		_, _ = a.pool.Exec(r.Context(), `
			INSERT INTO video_tags (video_id, tag_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, id, tagID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": tags})
}
