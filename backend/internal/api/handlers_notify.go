package api

import "net/http"

// POST /api/admin/notify/test — send a test notification.
func (a *App) testNotification(w http.ResponseWriter, r *http.Request) {
	a.notify.Send(r.Context(), "test", "VideoCMS test notification",
		"This is a test message from VideoCMS.", map[string]any{"source": "admin"})
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}
