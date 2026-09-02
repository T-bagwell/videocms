package api

import "net/http"

// GET /api/openapi.json — OpenAPI 3 description of the REST API.
func (a *App) openAPI(w http.ResponseWriter, r *http.Request) {
	paths := map[string]any{}
	add := func(path string, methods map[string]string) {
		m := map[string]any{}
		for verb, summary := range methods {
			m[verb] = map[string]any{"summary": summary}
		}
		paths[path] = m
	}

	add("/api/auth/login", map[string]string{"post": "Log in and get a JWT"})
	add("/api/auth/register", map[string]string{"post": "Register a new account"})
	add("/api/auth/registration", map[string]string{"get": "Current registration policy"})
	add("/api/auth/sso", map[string]string{"get": "SSO provider availability"})
	add("/api/auth/oidc/start", map[string]string{"get": "Start OIDC login"})
	add("/api/auth/saml/login", map[string]string{"get": "Start SAML login"})

	add("/api/libraries", map[string]string{"get": "List libraries", "post": "Create library"})
	add("/api/libraries/{id}", map[string]string{"patch": "Update library (block, quota)", "delete": "Delete library"})
	add("/api/libraries/{id}/scan", map[string]string{"post": "Trigger a scan"})

	add("/api/videos", map[string]string{"get": "List/search videos (q, library_id, tag, sort=fuzzy)"})
	add("/api/videos/{id}", map[string]string{"get": "Video detail", "patch": "Update video metadata (admin)"})
	add("/api/videos/{id}/versions", map[string]string{"get": "Multi-version copies of the film"})
	add("/api/videos/{id}/stream", map[string]string{"get": "HTTP Range stream"})
	add("/api/videos/{id}/download", map[string]string{"get": "Download original file"})
	add("/api/videos/{id}/download/remux", map[string]string{"get": "Download with selected tracks"})
	add("/api/videos/{id}/tracks", map[string]string{"get": "Audio/subtitle track list"})
	add("/api/videos/{id}/transcripts", map[string]string{"get": "Transcript status"})
	add("/api/videos/{id}/transcribe", map[string]string{"post": "Queue transcription (admin)"})
	add("/api/videos/{id}/subtitles", map[string]string{"get": "Active subtitle", "post": "Upload subtitle (admin)", "delete": "Remove subtitle (admin)"})
	add("/api/videos/{id}/subtitle-tracks", map[string]string{"get": "List subtitle tracks"})
	add("/api/videos/{id}/subtitles/{trackId}", map[string]string{"get": "Subtitle track content"})
	add("/api/videos/{id}/subtitles/{trackId}/active", map[string]string{"put": "Set active subtitle track"})
	add("/api/videos/{id}/subtitles/{trackId}/default", map[string]string{"put": "Set global default (admin)"})
	add("/api/videos/{id}/subtitles/preference", map[string]string{"delete": "Clear subtitle preference"})
	add("/api/videos/{id}/subtitles/search", map[string]string{"post": "Search online subtitles (admin)"})
	add("/api/videos/{id}/subtitles/download", map[string]string{"post": "Download a subtitle candidate (admin)"})
	add("/api/videos/{id}/subtitles/extract", map[string]string{"post": "Extract embedded subtitles (admin)"})
	add("/api/videos/{id}/poster", map[string]string{"get": "Serve poster", "post": "Upload poster (admin)"})
	add("/api/videos/{id}/backdrop", map[string]string{"get": "Serve backdrop", "post": "Upload backdrop (admin)"})
	add("/api/videos/{id}/thumbnails", map[string]string{"get": "Thumbnail strip metadata"})
	add("/api/videos/{id}/thumbnails/{n}", map[string]string{"get": "Thumbnail image"})
	add("/api/videos/{id}/hls/{file...}", map[string]string{"get": "HLS master/segments"})
	add("/api/videos/{id}/scrape", map[string]string{"post": "Scrape metadata (admin)"})
	add("/api/videos/{id}/tags", map[string]string{"get": "List video tags", "post": "Add tag"})
	add("/api/videos/{id}/tags/{tagId}", map[string]string{"delete": "Remove tag"})
	add("/api/videos/{id}/analyze", map[string]string{"post": "AI tag analysis (admin)"})
	add("/api/videos/{id}/comments", map[string]string{"get": "List comments", "post": "Post comment"})
	add("/api/videos/{id}/rating", map[string]string{"put": "Rate 1-5 stars"})
	add("/api/videos/{id}/ratings", map[string]string{"get": "Average + caller rating"})
	add("/api/videos/{id}/reaction", map[string]string{"put": "Like (1), dislike (-1), clear (0)"})
	add("/api/videos/{id}/reactions", map[string]string{"get": "Like/dislike counts"})
	add("/api/videos/{id}/report", map[string]string{"post": "Report content"})
	add("/api/videos/{id}/featurettes", map[string]string{"get": "List featurettes", "post": "Upload featurette (admin)"})
	add("/api/videos/{id}/featurettes/{fid}", map[string]string{"delete": "Delete featurette (admin)"})
	add("/api/videos/{id}/featurettes/{fid}/stream", map[string]string{"get": "Stream featurette"})
	add("/api/videos/{id}/theme-song", map[string]string{"get": "Theme song", "post": "Upload theme song (admin)", "delete": "Delete theme song (admin)"})
	add("/api/videos/{id}/theme-song/stream", map[string]string{"get": "Stream theme song"})
	add("/api/videos/{id}/chapters", map[string]string{"get": "Chapter index"})
	add("/api/videos/{id}/similar", map[string]string{"get": "Similar videos"})

	add("/api/series", map[string]string{"get": "List series (library_id, visible)"})
	add("/api/series/{id}", map[string]string{"get": "Series detail + episodes"})
	add("/api/series/{id}/favorite", map[string]string{"post": "Favorite series", "delete": "Unfavorite series"})
	add("/api/series/{id}/subscribe", map[string]string{"put": "Subscribe", "delete": "Unsubscribe"})
	add("/api/series/{id}/poster", map[string]string{"get": "Series poster"})

	add("/api/users/me", map[string]string{"get": "Current user"})
	add("/api/users/me/progress", map[string]string{"put": "Save watch progress"})
	add("/api/users/me/playback-speed/{videoId}", map[string]string{"get": "Remembered playback speed"})
	add("/api/users/me/continue", map[string]string{"get": "Continue watching"})
	add("/api/users/me/favorites", map[string]string{"get": "Favorites", "post": "Add favorite"})
	add("/api/users/me/favorites/{videoId}", map[string]string{"delete": "Remove favorite"})
	add("/api/users/me/series-favorites", map[string]string{"get": "Favorite series"})
	add("/api/users/me/subscriptions", map[string]string{"get": "Subscribed series"})
	add("/api/users/me/stats", map[string]string{"get": "Watch statistics"})
	add("/api/users/me/stats/export", map[string]string{"get": "Export watch history CSV"})
	add("/api/users/me/notification-prefs", map[string]string{"get": "Notification preferences", "put": "Save notification preferences"})
	add("/api/users/me/filters", map[string]string{"get": "Saved browse filters", "put": "Save browse filter"})
	add("/api/users/me/pin", map[string]string{"put": "Set parental PIN"})
	add("/api/users/me/pin/verify", map[string]string{"post": "Verify PIN, get unlock token"})
	add("/api/users/me/subtitle-offset", map[string]string{"get": "Subtitle offset", "put": "Set offset", "delete": "Reset offset"})

	add("/api/playlists", map[string]string{"get": "List playlists", "post": "Create playlist"})
	add("/api/playlists/{id}", map[string]string{"get": "Playlist detail", "patch": "Rename/update", "delete": "Delete playlist"})
	add("/api/playlists/{id}/items", map[string]string{"post": "Add item"})
	add("/api/playlists/{id}/items/{videoId}", map[string]string{"delete": "Remove item"})
	add("/api/tags", map[string]string{"get": "Tag cloud"})
	add("/api/collections", map[string]string{"get": "Smart collections", "post": "Save collection"})
	add("/api/collections/{id}", map[string]string{"delete": "Delete collection"})

	add("/api/share", map[string]string{"get": "List shares", "post": "Create share"})
	add("/api/share/{token}", map[string]string{"get": "Share info", "delete": "Delete share"})
	add("/api/share/{token}/video/{videoId}/stream", map[string]string{"get": "Public stream"})
	add("/api/share/{token}/video/{videoId}/download", map[string]string{"get": "Public download"})
	add("/api/share/{token}/video/{videoId}/poster", map[string]string{"get": "Public poster"})
	add("/api/share/{token}/video/{videoId}/subtitles", map[string]string{"get": "Public subtitles"})
	add("/api/share/{token}/video/{videoId}/hls/{file...}", map[string]string{"get": "Public HLS"})

	add("/api/uploads", map[string]string{"get": "List uploads", "post": "Create upload session"})
	add("/api/uploads/{id}/chunk/{n}", map[string]string{"put": "Upload chunk"})
	add("/api/uploads/{id}/complete", map[string]string{"post": "Finish upload"})
	add("/api/uploads/{id}", map[string]string{"delete": "Cancel upload"})
	add("/api/downloads", map[string]string{"get": "List yt-dlp jobs", "post": "Queue a download"})
	add("/api/downloads/{id}", map[string]string{"delete": "Cancel job"})
	add("/api/downloads/{id}/retry", map[string]string{"post": "Retry job"})

	add("/api/live", map[string]string{"get": "List live streams", "post": "Create live stream"})
	add("/api/live/{id}", map[string]string{"get": "Live stream detail"})
	add("/api/live/{id}/start", map[string]string{"post": "Start live stream (admin)"})
	add("/api/live/{id}/stop", map[string]string{"post": "Stop live stream (admin)"})
	add("/api/live/{id}/hls/{file...}", map[string]string{"get": "Live HLS"})
	add("/api/live/{id}/chat", map[string]string{"get": "Chat messages", "post": "Send chat message"})
	add("/api/watch/rooms", map[string]string{"post": "Create watch room"})
	add("/api/watch/rooms/{id}", map[string]string{"get": "Room state", "put": "Update sync state"})

	add("/api/albums", map[string]string{"get": "Music albums"})
	add("/api/albums/{id}", map[string]string{"get": "Album + tracks"})
	add("/api/albums/{id}/poster", map[string]string{"get": "Album cover"})
	add("/api/books", map[string]string{"get": "Books & comics"})
	add("/api/books/{id}", map[string]string{"get": "Book detail"})
	add("/api/books/{id}/file", map[string]string{"get": "Raw book file"})
	add("/api/books/{id}/pages", map[string]string{"get": "CBZ page list"})
	add("/api/books/{id}/pages/{n}", map[string]string{"get": "CBZ page image"})
	add("/api/books/{id}/epub/spine", map[string]string{"get": "EPUB chapter spine"})
	add("/api/books/{id}/epub/resource/{path...}", map[string]string{"get": "EPUB internal resource"})
	add("/api/photo-albums", map[string]string{"get": "Photo albums"})
	add("/api/photos", map[string]string{"get": "Photos (album_id, library_id)"})
	add("/api/photos/{id}/file", map[string]string{"get": "Photo file"})

	add("/api/iptv/channels", map[string]string{"get": "IPTV channels", "post": "Create channel (admin)"})
	add("/api/iptv/channels/{id}", map[string]string{"patch": "Update channel (admin)", "delete": "Delete channel (admin)"})
	add("/api/iptv/channels/{id}/logo", map[string]string{"get": "Channel logo", "post": "Upload logo (admin)"})
	add("/api/iptv/channels/{id}/catchup", map[string]string{"get": "Catch-up recordings"})
	add("/api/iptv/import", map[string]string{"post": "Import M3U playlist (admin)"})
	add("/api/iptv/library-channel", map[string]string{"post": "Create library channel (admin)"})
	add("/api/iptv/channels.m3u", map[string]string{"get": "M3U output (?token=)"})
	add("/api/iptv/epg.xml", map[string]string{"get": "XMLTV EPG output (?token=)"})
	add("/api/iptv/epg/import", map[string]string{"post": "Import XMLTV EPG (admin)"})
	add("/api/iptv/library/{id}/stream", map[string]string{"get": "Continuous library channel stream"})
	add("/api/iptv/recordings/{id}/stream", map[string]string{"get": "Stream a recording"})
	add("/api/admin/recordings", map[string]string{"get": "List recordings", "post": "Schedule recording"})
	add("/api/admin/recordings/{id}", map[string]string{"delete": "Delete recording"})
	add("/api/admin/tuners", map[string]string{"get": "Configured tuners"})
	add("/api/admin/tuners/scan", map[string]string{"post": "Scan HDHomeRun lineups"})

	add("/api/requests", map[string]string{"get": "My requests", "post": "Submit request"})
	add("/api/requests/all", map[string]string{"get": "All requests (admin)"})
	add("/api/requests/{id}/decide", map[string]string{"post": "Approve/reject (admin)"})

	add("/api/admin/users", map[string]string{"get": "List users"})
	add("/api/admin/users/{id}", map[string]string{"patch": "Update user role/rating"})
	add("/api/admin/users/{id}/moderation", map[string]string{"patch": "Mute/block user"})
	add("/api/admin/users/bulk", map[string]string{"post": "Bulk user actions"})
	add("/api/admin/reports", map[string]string{"get": "Content reports"})
	add("/api/admin/reports/{id}/decide", map[string]string{"post": "Review/dismiss report"})
	add("/api/admin/blocked-titles", map[string]string{"get": "Blocked titles", "post": "Block a title"})
	add("/api/admin/blocked-titles/{id}", map[string]string{"delete": "Unblock title"})
	add("/api/admin/stats", map[string]string{"get": "Server stats"})
	add("/api/admin/stats/watch", map[string]string{"get": "Aggregate watch stats"})
	add("/api/admin/stats/watch/export", map[string]string{"get": "Full watch history CSV"})
	add("/api/admin/jobs", map[string]string{"get": "Unified jobs dashboard"})
	add("/api/admin/quality-profiles", map[string]string{"get": "Quality profiles", "post": "Create profile"})
	add("/api/admin/quality-profiles/{id}", map[string]string{"delete": "Delete profile"})
	add("/api/admin/quality-profiles/{id}/active", map[string]string{"post": "Activate profile"})
	add("/api/admin/quality-profiles/apply", map[string]string{"post": "Re-score libraries"})
	add("/api/admin/invites", map[string]string{"get": "Invite codes", "post": "Generate invites"})
	add("/api/admin/invites/{id}", map[string]string{"delete": "Revoke invite"})
	add("/api/admin/trakt/status", map[string]string{"get": "Trakt configuration status"})
	add("/api/admin/trakt/sync", map[string]string{"post": "Sync watch history to Trakt"})
	add("/api/admin/maintenance", map[string]string{"post": "Run maintenance (rescan/backup)"})
	add("/api/admin/trash", map[string]string{"get": "Recycle bin"})
	add("/api/admin/trash/{id}/restore", map[string]string{"post": "Restore from trash"})
	add("/api/admin/export/nfo", map[string]string{"post": "Export NFO files"})
	add("/api/admin/import/nfo", map[string]string{"post": "Import NFO metadata"})
	add("/api/admin/health-checks", map[string]string{"post": "Run media health checks"})
	add("/api/admin/thumbnails/regenerate", map[string]string{"post": "Regenerate thumbnails"})
	add("/api/admin/webhooks", map[string]string{"get": "List webhooks", "post": "Create webhook"})
	add("/api/admin/webhooks/{id}", map[string]string{"delete": "Delete webhook"})
	add("/api/admin/storage-pools", map[string]string{"get": "Storage pools", "post": "Create pool"})

	add("/api/public/videos", map[string]string{"get": "Public video library"})
	add("/api/public/videos/{id}", map[string]string{"get": "Public video detail"})
	add("/api/public/videos/{id}/unlock", map[string]string{"post": "Unlock password-protected video"})
	add("/api/public/videos/{id}/stream", map[string]string{"get": "Public stream"})
	add("/api/public/videos/{id}/poster", map[string]string{"get": "Public poster"})

	add("/api/feed", map[string]string{"get": "Activity feed (?type=)"})
	add("/api/healthz", map[string]string{"get": "Health check"})
	add("/api/openapi.json", map[string]string{"get": "This document"})

	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "VideoCMS API",
			"version":     "0.1.0",
			"description": "Self-hosted video resource management system. Authenticate with POST /api/auth/login and pass the JWT as a Bearer header; media endpoints also accept ?token=.",
		},
		"paths": paths,
	}
	writeJSON(w, http.StatusOK, doc)
}

// GET /api/docs — Swagger UI for the OpenAPI document.
func (a *App) apiDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>VideoCMS API Docs</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
<style>body{margin:0;background:#fafafa}</style></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>window.onload=()=>SwaggerUIBundle({url:'/api/openapi.json',dom_id:'#swagger-ui'})</script>
</body></html>`))
}
