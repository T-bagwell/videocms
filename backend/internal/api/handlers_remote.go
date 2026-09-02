package api

import "net/http"

// GET /api/remote-access — remote-access helpers (TURN credentials).
func (a *App) remoteAccess(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"turn": nil}
	if a.cfg.TurnServer != "" {
		resp["turn"] = map[string]any{
			"url":        a.cfg.TurnServer,
			"username":   a.cfg.TurnUsername,
			"credential": a.cfg.TurnPassword,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
