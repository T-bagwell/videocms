package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"videocms/backend/internal/media"
)

type videoAccessClaims struct {
	Kind    string    `json:"kind"`
	VideoID uuid.UUID `json:"video_id"`
	jwt.RegisteredClaims
}

type publicLibraryVideo struct {
	ID                uuid.UUID `json:"id"`
	Title             string    `json:"title"`
	Year              int       `json:"year"`
	DurationSec       float64   `json:"duration_sec"`
	Width             int       `json:"width"`
	Height            int       `json:"height"`
	VideoCodec        string    `json:"video_codec"`
	Container         string    `json:"container"`
	Genres            []string  `json:"genres"`
	HasPoster         bool      `json:"has_poster"`
	Visibility        string    `json:"visibility"`
	PasswordProtected bool      `json:"password_protected"`
}

// publicVideoCondition matches videos that may be exposed anonymously:
// available, not blocked by admin title filters and not in a blocked library.
func publicVideoCondition() string {
	return `v.available AND NOT EXISTS (
		SELECT 1 FROM libraries lb WHERE lb.id = v.library_id AND lb.blocked
	) AND ` + blockedTitlesCondition()
}

func (a *App) signVideoAccess(id uuid.UUID) (string, error) {
	claims := videoAccessClaims{
		Kind:    "video_access",
		VideoID: id,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.cfg.JWTSecret))
}

func (a *App) validVideoAccess(r *http.Request, id uuid.UUID) bool {
	tokenStr := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("vt")
	}
	if tokenStr == "" {
		return false
	}
	claims := &videoAccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(a.cfg.JWTSecret), nil
	})
	return err == nil && token.Valid && claims.Kind == "video_access" && claims.VideoID == id
}

// GET /api/public/videos
func (a *App) listPublicLibrary(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT v.id, v.title, v.year, v.duration_sec, v.width, v.height,
		       v.video_codec, v.container, v.genres, v.poster_path
		FROM videos v JOIN libraries l ON l.id=v.library_id
		WHERE v.visibility='public' AND %s ORDER BY lower(v.title), v.year DESC`,
		publicVideoCondition()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query public videos failed")
		return
	}
	defer rows.Close()
	items := []publicLibraryVideo{}
	for rows.Next() {
		var p publicLibraryVideo
		var poster string
		if err := rows.Scan(&p.ID, &p.Title, &p.Year, &p.DurationSec, &p.Width, &p.Height,
			&p.VideoCodec, &p.Container, &p.Genres, &poster); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan public video failed")
			return
		}
		p.HasPoster = poster != ""
		p.Visibility = "public"
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// publicLibraryVideoFor returns the public metadata + file path when the anonymous
// request is allowed to view the video.
func (a *App) publicLibraryVideoFor(w http.ResponseWriter, r *http.Request, id uuid.UUID) (publicLibraryVideo, string, bool) {
	var p publicLibraryVideo
	var filePath, poster, visibility, hash string
	err := a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT v.title, v.year, v.duration_sec, v.width, v.height,
		       v.video_codec, v.container, v.genres, v.poster_path,
		       v.file_path, v.visibility, v.access_password_hash
		FROM videos v JOIN libraries l ON l.id=v.library_id
		WHERE v.id=$1 AND %s`, publicVideoCondition()), id).
		Scan(&p.Title, &p.Year, &p.DurationSec, &p.Width, &p.Height,
			&p.VideoCodec, &p.Container, &p.Genres, &poster,
			&filePath, &visibility, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "video not found")
		return publicLibraryVideo{}, "", false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query public video failed")
		return publicLibraryVideo{}, "", false
	}
	p.ID = id
	p.HasPoster = poster != ""
	p.Visibility = visibility
	p.PasswordProtected = false
	if visibility == "private" {
		writeErr(w, http.StatusNotFound, "video not found")
		return publicLibraryVideo{}, "", false
	}
	if visibility == "password" && !a.validVideoAccess(r, id) {
		p.PasswordProtected = true
		writeErr(w, http.StatusUnauthorized, "password required")
		return publicLibraryVideo{}, "", false
	}
	_ = hash
	return p, filePath, true
}

// GET /api/public/videos/{id}
func (a *App) getPublicVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	p, _, ok := a.publicLibraryVideoFor(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// POST /api/public/videos/{id}/unlock
func (a *App) unlockPublicVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var hash string
	err = a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT v.access_password_hash FROM videos v
		WHERE v.id=$1 AND v.visibility='password' AND %s`, publicVideoCondition()), id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query video failed")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	token, err := a.signVideoAccess(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sign access token failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

// GET /api/public/videos/{id}/stream
func (a *App) streamPublicVideo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	_, filePath, ok := a.publicLibraryVideoFor(w, r, id)
	if !ok {
		return
	}
	media.ServeVideoFile(w, r, filePath, media.ContentTypeFor(filePath), "")
}

// GET /api/public/videos/{id}/poster
func (a *App) publicVideoPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	if _, _, ok := a.publicLibraryVideoFor(w, r, id); !ok {
		return
	}
	var poster string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT poster_path FROM videos WHERE id=$1`, id).Scan(&poster); err != nil || poster == "" {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, poster)
}
