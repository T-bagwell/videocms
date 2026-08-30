package api

import "net/http"

// GET /api/openapi.json — lightweight OpenAPI 3 description of the API.
func (a *App) openAPI(w http.ResponseWriter, r *http.Request) {
	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "VideoCMS API",
			"version": "0.1.0",
			"description": "Self-hosted video resource management system. " +
				"Authenticate with POST /api/auth/login and pass the JWT as a Bearer header; " +
				"media endpoints also accept ?token=.",
		},
		"paths": map[string]any{
			"/api/auth/login":                 map[string]any{"post": map[string]any{"summary": "Log in and get a JWT"}},
			"/api/libraries":                  map[string]any{"get": map[string]any{"summary": "List libraries"}, "post": map[string]any{"summary": "Create library"}},
			"/api/videos":                     map[string]any{"get": map[string]any{"summary": "List/search videos (q, library_id, tag, sort=fuzzy)"}},
			"/api/videos/{id}":                map[string]any{"get": map[string]any{"summary": "Video detail"}},
			"/api/videos/{id}/stream":         map[string]any{"get": map[string]any{"summary": "HTTP Range stream"}},
			"/api/videos/{id}/download":       map[string]any{"get": map[string]any{"summary": "Download original"}},
			"/api/videos/{id}/download/remux": map[string]any{"get": map[string]any{"summary": "Download with selected tracks"}},
			"/api/uploads":                    map[string]any{"get": map[string]any{"summary": "List upload sessions"}, "post": map[string]any{"summary": "Create upload session"}},
			"/api/downloads":                  map[string]any{"get": map[string]any{"summary": "List yt-dlp jobs"}, "post": map[string]any{"summary": "Queue a download"}},
			"/api/watch/rooms":                map[string]any{"post": map[string]any{"summary": "Create watch room"}},
			"/api/live":                       map[string]any{"get": map[string]any{"summary": "List live streams"}, "post": map[string]any{"summary": "Create live stream"}},
			"/api/admin/users":                map[string]any{"get": map[string]any{"summary": "List users"}},
			"/api/admin/stats":                map[string]any{"get": map[string]any{"summary": "Stats"}},
			"/api/admin/jobs":                 map[string]any{"get": map[string]any{"summary": "Unified jobs dashboard"}},
			"/api/admin/storage-pools":        map[string]any{"get": map[string]any{"summary": "List storage pools"}, "post": map[string]any{"summary": "Create storage pool"}},
			"/api/healthz":                    map[string]any{"get": map[string]any{"summary": "Health check"}},
		},
	}
	writeJSON(w, http.StatusOK, doc)
}
