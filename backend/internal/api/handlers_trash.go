package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type trashRecord struct {
	ID           uuid.UUID  `json:"id"`
	OriginalPath string     `json:"original_path"`
	TrashPath    string     `json:"trash_path"`
	VideoID      *uuid.UUID `json:"video_id,omitempty"`
	MovedAt      time.Time  `json:"moved_at"`
}

// moveToTrash relocates a media file into DATA_DIR/trash/<date>/ and records
// the move so it can be restored.
func (a *App) moveToTrash(ctx context.Context, videoID uuid.UUID, path string) (string, error) {
	trashDir := filepath.Join(a.cfg.DataDir, "trash", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid media path")
	}
	base := filepath.Base(path)
	if base == "." || base == ".." {
		return "", fmt.Errorf("invalid media path")
	}
	src := filepath.Clean("/" + path)
	dst := filepath.Join(trashDir, base)
	if _, err := os.Stat(dst); err == nil {
		dst = filepath.Join(trashDir, fmt.Sprintf("%s-%s", videoID.String()[:8], base))
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	if _, err := a.pool.Exec(ctx, `
		INSERT INTO trash_records (original_path, trash_path, video_id)
		VALUES ($1, $2, $3)`, path, dst, videoID); err != nil {
		return dst, err
	}
	if _, err := a.pool.Exec(ctx,
		`UPDATE videos SET available=false WHERE id=$1`, videoID); err != nil {
		return dst, err
	}
	return dst, nil
}

// GET /api/admin/trash — list trashed files.
func (a *App) listTrash(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, original_path, trash_path, video_id, moved_at
		FROM trash_records ORDER BY moved_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list trash failed")
		return
	}
	defer rows.Close()
	items := []trashRecord{}
	for rows.Next() {
		var t trashRecord
		if err := rows.Scan(&t.ID, &t.OriginalPath, &t.TrashPath, &t.VideoID, &t.MovedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan trash failed")
			return
		}
		items = append(items, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/trash/{id}/restore — move the file back and re-enable it.
func (a *App) restoreTrash(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid trash id")
		return
	}
	var t trashRecord
	err = a.pool.QueryRow(r.Context(), `
		SELECT id, original_path, trash_path, video_id, moved_at
		FROM trash_records WHERE id=$1`, id).Scan(&t.ID, &t.OriginalPath, &t.TrashPath, &t.VideoID, &t.MovedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "trash record not found")
		return
	}
	if err := os.Rename(t.TrashPath, t.OriginalPath); err != nil {
		writeErr(w, http.StatusBadGateway, "restore failed: "+err.Error())
		return
	}
	if t.VideoID != nil {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE videos SET available=true, file_path=$2 WHERE id=$1`,
			*t.VideoID, t.OriginalPath); err != nil {
			writeErr(w, http.StatusInternalServerError, "re-enable video failed")
			return
		}
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM trash_records WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "clear trash record failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"restored": t.OriginalPath})
}

type batchVideosRequest struct {
	IDs    []uuid.UUID `json:"ids"`
	Action string      `json:"action"`
	Tag    string      `json:"tag"`
}

// POST /api/admin/videos/batch — bulk tag, clear tags or move to trash.
func (a *App) batchVideos(w http.ResponseWriter, r *http.Request) {
	var req batchVideosRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, "ids are required")
		return
	}
	switch req.Action {
	case "tag":
		name := strings.ToLower(strings.TrimSpace(req.Tag))
		if name == "" {
			writeErr(w, http.StatusBadRequest, "tag is required")
			return
		}
		tagID, err := a.ensureTag(r, name, "manual")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "create tag failed")
			return
		}
		for _, id := range req.IDs {
			_, _ = a.pool.Exec(r.Context(), `
				INSERT INTO video_tags (video_id, tag_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, id, tagID)
		}
	case "clear_tags":
		_, _ = a.pool.Exec(r.Context(), `
			DELETE FROM video_tags WHERE video_id = ANY($1)`, req.IDs)
	case "delete":
		for _, id := range req.IDs {
			var path string
			if err := a.pool.QueryRow(r.Context(),
				`SELECT file_path FROM videos WHERE id=$1`, id).Scan(&path); err != nil {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				_, _ = a.moveToTrash(r.Context(), id, path)
			} else {
				_, _ = a.pool.Exec(r.Context(),
					`UPDATE videos SET available=false WHERE id=$1`, id)
			}
		}
	default:
		writeErr(w, http.StatusBadRequest, "action must be tag, clear_tags or delete")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
