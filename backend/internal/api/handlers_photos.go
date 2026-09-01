package api

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

func visiblePhotosCondition(userParam int) string {
	return fmt.Sprintf(`p.available AND NOT EXISTS (
		SELECT 1 FROM hidden_paths hp WHERE hp.user_id=$%d
		  AND (p.file_path = hp.path OR starts_with(p.file_path, hp.path || '/'))
	) AND NOT EXISTS (SELECT 1 FROM libraries lb WHERE lb.id = p.library_id AND lb.blocked)`, userParam)
}

// GET /api/photo-albums
func (a *App) listPhotoAlbums(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFrom(r).ID
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT pa.id, pa.library_id, pa.name,
		       (SELECT count(*) FROM photos p WHERE p.album_id=pa.id AND %s) AS photo_count,
		       (SELECT p.id FROM photos p WHERE p.album_id=pa.id AND p.available
		        ORDER BY p.created_at LIMIT 1) AS cover_photo_id,
		       pa.created_at
		FROM photo_albums pa
		WHERE EXISTS (SELECT 1 FROM photos p WHERE p.album_id=pa.id AND %s)
		ORDER BY lower(pa.name)`,
		visiblePhotosCondition(1), visiblePhotosCondition(1)), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query photo albums failed")
		return
	}
	defer rows.Close()
	items := []models.PhotoAlbum{}
	for rows.Next() {
		var al models.PhotoAlbum
		if err := rows.Scan(&al.ID, &al.LibraryID, &al.Name, &al.PhotoCount,
			&al.CoverPhotoID, &al.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan photo album failed")
			return
		}
		al.HasCover = al.CoverPhotoID != nil
		items = append(items, al)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/photos?album_id=&library_id=
func (a *App) listPhotos(w http.ResponseWriter, r *http.Request) {
	args := []any{auth.UserFrom(r).ID}
	where := visiblePhotosCondition(1)
	if albumID := r.URL.Query().Get("album_id"); albumID != "" {
		if id, err := uuid.Parse(albumID); err == nil {
			where += fmt.Sprintf(` AND p.album_id=$%d`, len(args)+1)
			args = append(args, id)
		}
	}
	if libID := r.URL.Query().Get("library_id"); libID != "" {
		if id, err := uuid.Parse(libID); err == nil {
			where += fmt.Sprintf(` AND p.library_id=$%d`, len(args)+1)
			args = append(args, id)
		}
	}
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT p.id, p.album_id, p.title, p.width, p.height, p.size_bytes,
		       p.taken_at, p.camera, p.created_at
		FROM photos p WHERE %s ORDER BY p.created_at, p.title`, where), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query photos failed")
		return
	}
	defer rows.Close()
	items := []models.Photo{}
	for rows.Next() {
		var p models.Photo
		if err := rows.Scan(&p.ID, &p.AlbumID, &p.Title, &p.Width, &p.Height,
			&p.SizeBytes, &p.TakenAt, &p.Camera, &p.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan photo failed")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/photos/{id}/file
func (a *App) photoFile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid photo id")
		return
	}
	var filePath string
	err = a.pool.QueryRow(r.Context(), fmt.Sprintf(
		`SELECT p.file_path FROM photos p WHERE p.id=$1 AND %s`, visiblePhotosCondition(2)),
		id, auth.UserFrom(r).ID).Scan(&filePath)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "photo not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query photo failed")
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(filePath)))
	http.ServeFile(w, r, filePath)
}
