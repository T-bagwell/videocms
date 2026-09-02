package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/auth"
)

type commentItem struct {
	ID        uuid.UUID `json:"id"`
	VideoID   uuid.UUID `json:"video_id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// GET /api/videos/{id}/comments
func (a *App) listComments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT c.id, c.video_id, c.user_id, u.display_name, c.body, c.created_at
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.video_id=$1 AND NOT u.muted AND NOT u.global_blocked
		ORDER BY c.created_at LIMIT 200`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list comments failed")
		return
	}
	defer rows.Close()
	items := []commentItem{}
	for rows.Next() {
		var c commentItem
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.Body, &c.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan comment failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/videos/{id}/comments
func (a *App) addComment(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var allowComments bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT allow_comments FROM videos WHERE id=$1`, id).Scan(&allowComments); err != nil || !allowComments {
		writeErr(w, http.StatusForbidden, "comments are disabled for this video")
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" || len(req.Body) > 1000 {
		writeErr(w, http.StatusBadRequest, "comment must be 1-1000 characters")
		return
	}
	var muted, blocked bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT muted, global_blocked FROM users WHERE id=$1`, user.ID).Scan(&muted, &blocked); err == nil &&
		(muted || blocked) {
		writeErr(w, http.StatusForbidden, "your account is muted or blocked from commenting")
		return
	}
	var c commentItem
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO comments (video_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, video_id, user_id, body, created_at`,
		id, user.ID, req.Body).Scan(&c.ID, &c.VideoID, &c.UserID, &c.Body, &c.CreatedAt); err != nil {
		writeErr(w, http.StatusBadRequest, "video not found")
		return
	}
	c.Username = user.DisplayName
	writeJSON(w, http.StatusCreated, c)
}

// DELETE /api/comments/{id} — the author or an admin.
func (a *App) deleteComment(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid comment id")
		return
	}
	tag, err := a.pool.Exec(r.Context(), `
		DELETE FROM comments WHERE id=$1 AND (user_id=$2 OR $3='admin')`,
		id, user.ID, user.Role)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete comment failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusForbidden, "cannot delete this comment")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// GET /api/videos/{id}/ratings — average + caller's rating.
func (a *App) getRatings(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var avg float64
	var count int
	_ = a.pool.QueryRow(r.Context(), `
		SELECT COALESCE(AVG(stars), 0), COUNT(*) FROM ratings WHERE video_id=$1`, id).Scan(&avg, &count)
	mine := 0
	_ = a.pool.QueryRow(r.Context(), `
		SELECT stars FROM ratings WHERE video_id=$1 AND user_id=$2`, id, user.ID).Scan(&mine)
	writeJSON(w, http.StatusOK, map[string]any{"average": avg, "count": count, "mine": mine})
}

// PUT /api/videos/{id}/rating
func (a *App) rateVideo(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req struct {
		Stars int `json:"stars"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Stars < 1 || req.Stars > 5 {
		writeErr(w, http.StatusBadRequest, "stars must be 1-5")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO ratings (video_id, user_id, stars)
		VALUES ($1, $2, $3)
		ON CONFLICT (video_id, user_id)
		DO UPDATE SET stars=EXCLUDED.stars, updated_at=now()`,
		id, user.ID, req.Stars); err != nil {
		writeErr(w, http.StatusBadRequest, "video not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stars": req.Stars})
}

type feedItem struct {
	Kind       string     `json:"kind"`
	Username   string     `json:"username"`
	Text       string     `json:"text"`
	VideoID    *uuid.UUID `json:"video_id"`
	VideoTitle string     `json:"video_title"`
	SeriesID   *uuid.UUID `json:"series_id"`
	SeriesName string     `json:"series_name"`
	CreatedAt  time.Time  `json:"created_at"`
}

// GET /api/feed — filterable recent activity (comments, favorites, ratings,
// subscriptions). Optional ?type= filters by kind.
func (a *App) feed(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimSpace(r.URL.Query().Get("type"))
	rows, err := a.pool.Query(r.Context(), `
		SELECT t.kind, COALESCE(t.username,''), COALESCE(t.text,''), t.video_id,
		       COALESCE(t.video_title,''), t.series_id, COALESCE(t.series_name,''), t.created_at
		FROM (
		(SELECT 'comment' AS kind, u.display_name AS username, c.body AS text,
		        v.id AS video_id, v.title AS video_title,
		        NULL::uuid AS series_id, NULL::text AS series_name, c.created_at AS created_at
		 FROM comments c JOIN users u ON u.id=c.user_id JOIN videos v ON v.id=c.video_id)
		UNION ALL
		(SELECT 'favorite', u.display_name, '', v.id, v.title, NULL, NULL, f.created_at
		 FROM favorites f JOIN users u ON u.id=f.user_id JOIN videos v ON v.id=f.video_id)
		UNION ALL
		(SELECT 'rating', u.display_name, '★' || r.stars::text, v.id, v.title, NULL, NULL, r.created_at
		 FROM ratings r JOIN users u ON u.id=r.user_id JOIN videos v ON v.id=r.video_id)
		UNION ALL
		(SELECT 'subscription', u.display_name, '', NULL, NULL, s.id, s.name, cs.created_at
		 FROM channel_subscriptions cs JOIN users u ON u.id=cs.user_id JOIN series s ON s.id=cs.series_id)
		) t WHERE ($1 = '' OR t.kind = $1)
		ORDER BY created_at DESC LIMIT 30`, kind)
	if err != nil {
		log.Printf("feed query: %v", err)
		writeErr(w, http.StatusInternalServerError, "feed failed")
		return
	}
	defer rows.Close()
	items := []feedItem{}
	for rows.Next() {
		var f feedItem
		if err := rows.Scan(&f.Kind, &f.Username, &f.Text, &f.VideoID, &f.VideoTitle,
			&f.SeriesID, &f.SeriesName, &f.CreatedAt); err != nil {
			log.Printf("feed scan: %v", err)
			writeErr(w, http.StatusInternalServerError, "scan feed failed")
			return
		}
		items = append(items, f)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
