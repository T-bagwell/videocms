package api

import (
	"net/http"
	"strings"

	"videocms/backend/internal/auth"
)

// GET /api/users/me/notification-prefs
func (a *App) getNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFrom(r).ID
	var enabled bool
	var events []string
	err := a.pool.QueryRow(r.Context(), `
		SELECT enabled, events FROM user_notification_prefs WHERE user_id=$1`, userID).
		Scan(&enabled, &events)
	if err != nil {
		enabled = true
		events = []string{"scan", "download", "comment", "favorite", "rating", "subscription", "new_episode"}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "events": events})
}

// PUT /api/users/me/notification-prefs
func (a *App) setNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool    `json:"enabled"`
		Events  []string `json:"events"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := auth.UserFrom(r).ID
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	events := req.Events
	for i := range events {
		events[i] = strings.TrimSpace(events[i])
	}
	if _, err := a.pool.Exec(r.Context(), `
		INSERT INTO user_notification_prefs (user_id, enabled, events, updated_at)
		VALUES ($1,$2,$3,now())
		ON CONFLICT (user_id) DO UPDATE SET
			enabled=EXCLUDED.enabled, events=EXCLUDED.events, updated_at=now()`,
		userID, enabled, events); err != nil {
		writeErr(w, http.StatusInternalServerError, "save notification prefs failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "events": events})
}
