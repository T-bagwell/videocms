package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/models"
)

type qualityProfile struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	MinHeight      int       `json:"min_height"`
	MaxHeight      int       `json:"max_height"`
	PreferredCodec string    `json:"preferred_codec"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func scanQualityProfile(row pgx.Row) (qualityProfile, error) {
	var p qualityProfile
	err := row.Scan(&p.ID, &p.Name, &p.MinHeight, &p.MaxHeight, &p.PreferredCodec,
		&p.Active, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// GET /api/admin/quality-profiles
func (a *App) listQualityProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, min_height, max_height, preferred_codec, active, created_at, updated_at
		FROM quality_profiles ORDER BY active DESC, lower(name)`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query quality profiles failed")
		return
	}
	defer rows.Close()
	items := []qualityProfile{}
	for rows.Next() {
		p, err := scanQualityProfile(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan quality profile failed")
			return
		}
		items = append(items, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/quality-profiles
func (a *App) createQualityProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		MinHeight      int    `json:"min_height"`
		MaxHeight      int    `json:"max_height"`
		PreferredCodec string `json:"preferred_codec"`
		Active         bool   `json:"active"`
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
	if req.MinHeight < 0 {
		req.MinHeight = 0
	}
	if req.MaxHeight < 0 {
		req.MaxHeight = 0
	}
	var p qualityProfile
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO quality_profiles (name, min_height, max_height, preferred_codec, active)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, name, min_height, max_height, preferred_codec, active, created_at, updated_at`,
		req.Name, req.MinHeight, req.MaxHeight, strings.ToLower(strings.TrimSpace(req.PreferredCodec)), req.Active).
		Scan(&p.ID, &p.Name, &p.MinHeight, &p.MaxHeight, &p.PreferredCodec, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create quality profile failed")
		return
	}
	if p.Active {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE quality_profiles SET active=false WHERE id <> $1`, p.ID); err != nil {
			log.Printf("deactivate other profiles: %v", err)
		}
	}
	writeJSON(w, http.StatusCreated, p)
}

// DELETE /api/admin/quality-profiles/{id}
func (a *App) deleteQualityProfile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid profile id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM quality_profiles WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete quality profile failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// POST /api/admin/quality-profiles/{id}/active
func (a *App) setActiveQualityProfile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid profile id")
		return
	}
	var exists bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM quality_profiles WHERE id=$1)`, id).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "profile not found")
		return
	}
	tx, err := a.pool.Begin(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "begin transaction failed")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if _, err := tx.Exec(r.Context(),
		`UPDATE quality_profiles SET active=false`); err != nil {
		writeErr(w, http.StatusInternalServerError, "deactivate profiles failed")
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE quality_profiles SET active=true, updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "activate profile failed")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, "commit failed")
		return
	}
	a.applyQualityProfileNow(r)
	writeJSON(w, http.StatusOK, map[string]any{"message": "activated"})
}

// POST /api/admin/quality-profiles/apply
func (a *App) applyQualityProfile(w http.ResponseWriter, r *http.Request) {
	n := a.applyQualityProfileNow(r)
	writeJSON(w, http.StatusOK, map[string]any{"applied": n})
}

func (a *App) applyQualityProfileNow(r *http.Request) int {
	rows, err := a.pool.Query(r.Context(),
		`SELECT id, name, path FROM libraries`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var lib models.Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.Path); err != nil {
			continue
		}
		a.scanner.RebuildMovieVersions(r.Context(), lib)
		n++
	}
	return n
}
