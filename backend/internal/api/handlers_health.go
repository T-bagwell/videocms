package api

import (
	"net/http"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

// POST /api/libraries/{id}/health — run a health check on the library.
func (a *App) runHealthCheck(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	report, err := media.RunHealthCheck(r.Context(), a.pool, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "health check failed")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// POST /api/libraries/{id}/health/keep-best — move duplicates to trash except
// the best version of each group.
func (a *App) keepBestVersions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library id")
		return
	}
	moved, errorsList, err := media.KeepBestMoves(r.Context(), a.pool, a.cfg.DataDir, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "keep best failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"moved":  moved,
		"errors": errorsList,
	})
}
