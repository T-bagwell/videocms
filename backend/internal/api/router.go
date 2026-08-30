package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/crewjam/saml"
	"github.com/jackc/pgx/v5/pgxpool"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/config"
	"videocms/backend/internal/media"
)

type App struct {
	cfg         config.Config
	pool        *pgxpool.Pool
	scanner     *media.Scanner
	hls         *media.HLSManager
	scraper     *media.Scraper
	dl          *media.Downloader
	subProvider media.SubtitleProvider
	live        *media.LiveManager
	notify      *media.Notifier
	dlna        *media.DLNAManager
	samlMu      sync.Mutex
	samlSP      *saml.ServiceProvider
}

func New(cfg config.Config, pool *pgxpool.Pool) (*App, error) {
	// A server restart leaves any in-flight scan marked "scanning"; reset it.
	if _, err := pool.Exec(context.Background(),
		`UPDATE libraries SET scan_status='error', scan_error='Server restarted, scan interrupted; please rescan'
		 WHERE scan_status='scanning'`); err != nil {
		return nil, err
	}
	scanner := media.NewScanner(pool, cfg.DataDir)
	notify := media.NewNotifier(cfg.NotifyWebhookURL, cfg.NotifyAppriseURL, media.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		User:     cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		From:     cfg.NotifyEmailFrom,
		To:       cfg.NotifyEmailTo,
	})
	var dlnaMgr *media.DLNAManager
	if cfg.DLNAEnabled {
		port := cfg.Addr
		port = strings.TrimPrefix(port, ":")
		dlnaMgr = media.NewDLNAManager(cfg.DLNAFriendlyName, port, cfg.DLNAAllowedIPs)
	}
	app := &App{
		cfg:     cfg,
		pool:    pool,
		scanner: scanner,
		hls:     media.NewHLSManager(cfg.DataDir, media.ResolveTool("ffmpeg"), cfg.HLSHWAccel),
		scraper: media.NewScraper(pool, cfg.DataDir, cfg.TMDBAPIKey, cfg.ScrapeCustomURL),
		dl:      media.NewDownloader(pool, cfg.YtDLPPath),
		live:    media.NewLiveManager(cfg.DataDir, media.ResolveTool("ffmpeg")),
		notify:  notify,
		dlna:    dlnaMgr,
	}
	app.hls.SetVAAPIDevice(cfg.HLSVAAPIDevice)
	app.hls.SetToneMap(cfg.HLSToneMap)
	scanner.SetEnricher(app.scraper)
	scanner.SetNotify(func(name, status string) {
		event := "scan.completed"
		if status == "error" {
			event = "scan.failed"
		}
		app.notifyEvent(event, "Scan "+name, "Library "+name+" finished with status "+status,
			map[string]any{"library": name, "status": status})
	})
	app.dl.SetNotify(func(url string, err error) {
		event := "download.completed"
		title, body := "Download completed", url
		if err != nil {
			event, title, body = "download.failed", "Download failed", url+": "+err.Error()
		}
		app.notifyEvent(event, title, body, map[string]any{"url": url})
	})
	return app, nil
}

// StartDownloadWorker runs the yt-dlp background worker until ctx is done.
func (a *App) StartDownloadWorker(ctx context.Context) {
	go a.dl.Run(ctx)
}

// StartDLNA runs the SSDP responder when the feature is enabled.
func (a *App) StartDLNA(ctx context.Context) {
	if a.dlna != nil {
		a.dlna.Start(ctx)
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	authUser := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(a.pool, a.cfg.JWTSecret, next)
	}
	authAdmin := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAdmin(a.pool, a.cfg.JWTSecret, next)
	}

	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("GET /api/auth/sso", a.ssoStatus)
	mux.HandleFunc("GET /api/auth/me", authUser(a.me))
	mux.HandleFunc("GET /api/auth/oidc/start", a.oidcStart)
	mux.HandleFunc("GET /api/auth/oidc/callback", a.oidcCallback)
	mux.HandleFunc("GET /api/auth/saml/login", a.samlLogin)
	mux.HandleFunc("POST /api/auth/saml/acs", a.samlACS)
	mux.HandleFunc("GET /api/auth/saml/metadata", a.samlMetadata)

	if a.dlna != nil {
		mux.HandleFunc("GET /dlna/device.xml", a.dlnaGuard(a.dlnaDeviceDescription))
		mux.HandleFunc("GET /dlna/scpd.xml", a.dlnaGuard(a.dlnaSCPD))
		mux.HandleFunc("GET /dlna/content/{id}", a.dlnaGuard(a.dlnaBrowseGET))
		mux.HandleFunc("POST /dlna/control/ContentDirectory", a.dlnaGuard(a.dlnaContentControl))
		mux.HandleFunc("GET /dlna/video/{id}/stream", a.dlnaGuard(a.dlnaVideoStream))
		mux.HandleFunc("GET /dlna/video/{id}/poster", a.dlnaGuard(a.dlnaVideoPoster))
	}

	mux.HandleFunc("GET /api/libraries", authUser(a.listLibraries))
	mux.HandleFunc("POST /api/libraries", authAdmin(a.createLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", authAdmin(a.scanLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan/cancel", authAdmin(a.cancelScan))
	mux.HandleFunc("POST /api/libraries/{id}/open", authAdmin(a.openLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/health", authAdmin(a.runHealthCheck))
	mux.HandleFunc("POST /api/libraries/{id}/health/keep-best", authAdmin(a.keepBestVersions))
	mux.HandleFunc("POST /api/libraries/{id}/export-nfo", authAdmin(a.exportLibraryNFO))
	mux.HandleFunc("POST /api/libraries/{id}/import-nfo", authAdmin(a.importLibraryNFO))

	mux.HandleFunc("GET /api/admin/trash", authAdmin(a.listTrash))
	mux.HandleFunc("POST /api/admin/trash/{id}/restore", authAdmin(a.restoreTrash))
	mux.HandleFunc("POST /api/admin/videos/batch", authAdmin(a.batchVideos))
	mux.HandleFunc("POST /api/admin/notify/test", authAdmin(a.testNotification))
	mux.HandleFunc("GET /api/admin/storage-pools", authAdmin(a.listStoragePoolsAdmin))
	mux.HandleFunc("POST /api/admin/storage-pools", authAdmin(a.createStoragePool))
	mux.HandleFunc("PATCH /api/admin/storage-pools/{id}", authAdmin(a.updateStoragePool))
	mux.HandleFunc("DELETE /api/admin/storage-pools/{id}", authAdmin(a.deleteStoragePool))
	mux.HandleFunc("GET /api/storage-pools", authUser(a.listStoragePoolsUser))
	mux.HandleFunc("PATCH /api/libraries/{id}", authAdmin(a.setLibraryBlocked))
	mux.HandleFunc("DELETE /api/libraries/{id}", authAdmin(a.deleteLibrary))

	mux.HandleFunc("GET /api/uploads", authAdmin(a.listUploads))
	mux.HandleFunc("POST /api/uploads", authAdmin(a.createUpload))
	mux.HandleFunc("GET /api/uploads/{id}", authAdmin(a.getUpload))
	mux.HandleFunc("PUT /api/uploads/{id}/chunk/{index}", authAdmin(a.putChunk))
	mux.HandleFunc("POST /api/uploads/{id}/complete", authAdmin(a.completeUpload))
	mux.HandleFunc("DELETE /api/uploads/{id}", authAdmin(a.deleteUpload))

	mux.HandleFunc("GET /api/downloads", authAdmin(a.listDownloads))
	mux.HandleFunc("POST /api/downloads", authAdmin(a.createDownload))
	mux.HandleFunc("DELETE /api/downloads/{id}", authAdmin(a.deleteDownload))
	mux.HandleFunc("POST /api/downloads/{id}/retry", authAdmin(a.retryDownload))

	mux.HandleFunc("GET /api/admin/blocked-titles", authAdmin(a.listBlockedTitles))
	mux.HandleFunc("POST /api/admin/blocked-titles", authAdmin(a.createBlockedTitle))
	mux.HandleFunc("DELETE /api/admin/blocked-titles/{id}", authAdmin(a.deleteBlockedTitle))

	mux.HandleFunc("GET /api/videos", authUser(a.listVideos))
	mux.HandleFunc("GET /api/videos/{id}", authUser(a.getVideo))
	mux.HandleFunc("PATCH /api/videos/{id}", authAdmin(a.updateVideo))
	mux.HandleFunc("POST /api/videos/{id}/poster", authAdmin(a.uploadPoster))
	mux.HandleFunc("GET /api/videos/{id}/stream", authUser(a.streamVideo))
	mux.HandleFunc("GET /api/videos/{id}/download", authUser(a.downloadVideo))
	mux.HandleFunc("GET /api/videos/{id}/download/remux", authUser(a.remuxDownload))
	mux.HandleFunc("GET /api/videos/{id}/tracks", authUser(a.videoTracks))
	mux.HandleFunc("GET /api/videos/{id}/transcripts", authUser(a.getTranscript))
	mux.HandleFunc("POST /api/videos/{id}/transcribe", authAdmin(a.transcribeVideo))
	mux.HandleFunc("GET /api/videos/{id}/subtitles", authUser(a.subtitles))
	mux.HandleFunc("GET /api/videos/{id}/subtitle-tracks", authUser(a.listSubtitleTracks))
	mux.HandleFunc("GET /api/videos/{id}/subtitles/{trackId}", authUser(a.getSubtitleTrack))
	mux.HandleFunc("PUT /api/videos/{id}/subtitles/{trackId}/active", authUser(a.setActiveSubtitleTrack))
	mux.HandleFunc("DELETE /api/videos/{id}/subtitles/preference", authUser(a.clearSubtitlePreference))
	mux.HandleFunc("PUT /api/videos/{id}/subtitles/{trackId}/default", authAdmin(a.setGlobalSubtitleDefault))
	mux.HandleFunc("POST /api/videos/{id}/subtitles", authAdmin(a.uploadSubtitle))
	mux.HandleFunc("DELETE /api/videos/{id}/subtitles", authAdmin(a.deleteSubtitle))
	mux.HandleFunc("POST /api/videos/{id}/subtitles/extract", authAdmin(a.extractEmbeddedSubtitle))
	mux.HandleFunc("POST /api/videos/{id}/subtitles/search", authAdmin(a.searchSubtitles))
	mux.HandleFunc("POST /api/videos/{id}/subtitles/download", authAdmin(a.downloadSubtitle))
	mux.HandleFunc("GET /api/videos/{id}/poster", authUser(a.servePoster))
	mux.HandleFunc("GET /api/videos/{id}/thumbnails", authUser(a.videoThumbnails))
	mux.HandleFunc("GET /api/videos/{id}/thumbnails/{n}", authUser(a.videoThumbnailImage))
	mux.HandleFunc("GET /api/videos/{id}/hls/{file...}", authUser(a.hlsHandler))
	mux.HandleFunc("POST /api/videos/{id}/scrape", authAdmin(a.scrapeVideo))
	mux.HandleFunc("GET /api/videos/{id}/tags", authUser(a.listVideoTags))
	mux.HandleFunc("POST /api/videos/{id}/tags", authUser(a.addVideoTag))
	mux.HandleFunc("DELETE /api/videos/{id}/tags/{tagId}", authUser(a.removeVideoTag))
	mux.HandleFunc("POST /api/videos/{id}/analyze", authAdmin(a.analyzeVideo))
	mux.HandleFunc("GET /api/videos/{id}/similar", authUser(a.similarVideos))
	mux.HandleFunc("GET /api/tags", authUser(a.tagCloud))

	mux.HandleFunc("GET /api/collections", authUser(a.listCollections))
	mux.HandleFunc("POST /api/collections", authUser(a.createCollection))
	mux.HandleFunc("DELETE /api/collections/{id}", authUser(a.deleteCollection))
	mux.HandleFunc("GET /api/users/me/filters", authUser(a.getUserFilters))
	mux.HandleFunc("PUT /api/users/me/filters", authUser(a.saveUserFilters))
	mux.HandleFunc("PUT /api/users/me/pin", authUser(a.setPin))
	mux.HandleFunc("POST /api/users/me/pin/verify", authUser(a.verifyPin))

	mux.HandleFunc("GET /api/videos/{id}/comments", authUser(a.listComments))
	mux.HandleFunc("POST /api/videos/{id}/comments", authUser(a.addComment))
	mux.HandleFunc("DELETE /api/comments/{id}", authUser(a.deleteComment))
	mux.HandleFunc("GET /api/videos/{id}/ratings", authUser(a.getRatings))
	mux.HandleFunc("PUT /api/videos/{id}/rating", authUser(a.rateVideo))
	mux.HandleFunc("GET /api/feed", authUser(a.feed))
	mux.HandleFunc("POST /api/videos/{id}/share", authUser(a.createVideoShare))
	mux.HandleFunc("GET /api/videos/{id}/shares", authUser(a.listVideoShares))
	mux.HandleFunc("POST /api/series/{id}/share", authUser(a.createSeriesShare))
	mux.HandleFunc("GET /api/series/{id}/shares", authUser(a.listSeriesShares))
	mux.HandleFunc("POST /api/playlists/{id}/share", authUser(a.createPlaylistShare))
	mux.HandleFunc("GET /api/playlists/{id}/shares", authUser(a.listPlaylistShares))
	mux.HandleFunc("DELETE /api/share/{token}", authUser(a.deleteShare))
	mux.HandleFunc("GET /api/share/{token}/info", a.shareInfo)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/stream", a.shareStream)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/download", a.shareDownload)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/poster", a.sharePoster)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/subtitles", a.shareSubtitles)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/subtitle-tracks", a.shareSubtitleTracks)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/subtitles/{trackId}", a.shareSubtitleTrack)
	mux.HandleFunc("GET /api/share/{token}/video/{videoId}/hls/{file...}", a.shareHLS)

	mux.HandleFunc("GET /api/series", authUser(a.listSeries))
	mux.HandleFunc("GET /api/series/{id}", authUser(a.getSeries))
	mux.HandleFunc("GET /api/series/{id}/poster", authUser(a.seriesPoster))
	mux.HandleFunc("POST /api/series/{id}/favorite", authUser(a.addSeriesFavorite))
	mux.HandleFunc("DELETE /api/series/{id}/favorite", authUser(a.removeSeriesFavorite))
	mux.HandleFunc("GET /api/users/me/series-favorites", authUser(a.mySeriesFavorites))

	mux.HandleFunc("GET /api/admin/users", authAdmin(a.listUsers))
	mux.HandleFunc("PATCH /api/admin/users/{id}", authAdmin(a.updateUser))
	mux.HandleFunc("POST /api/admin/users/{id}/reset-password", authAdmin(a.resetPassword))
	mux.HandleFunc("DELETE /api/admin/users/{id}", authAdmin(a.deleteUser))

	mux.HandleFunc("PUT /api/users/me/progress", authUser(a.saveProgress))
	mux.HandleFunc("GET /api/users/me/continue", authUser(a.continueWatching))
	mux.HandleFunc("POST /api/users/me/favorites", authUser(a.addFavorite))
	mux.HandleFunc("DELETE /api/users/me/favorites/{videoId}", authUser(a.removeFavorite))
	mux.HandleFunc("GET /api/users/me/favorites", authUser(a.listFavorites))
	mux.HandleFunc("GET /api/users/me/hidden-paths", authUser(a.listHiddenPaths))
	mux.HandleFunc("POST /api/users/me/hidden-paths", authUser(a.addHiddenPath))
	mux.HandleFunc("DELETE /api/users/me/hidden-paths/{id}", authUser(a.removeHiddenPath))
	mux.HandleFunc("GET /api/users/me/subtitle-offset", authUser(a.getUserSubtitleOffset))
	mux.HandleFunc("PUT /api/users/me/subtitle-offset", authUser(a.setUserSubtitleOffset))
	mux.HandleFunc("DELETE /api/users/me/subtitle-offset", authUser(a.clearUserSubtitleOffset))
	mux.HandleFunc("GET /api/users/me/export", authUser(a.exportMe))

	mux.HandleFunc("POST /api/watch/rooms", authUser(a.createWatchRoom))
	mux.HandleFunc("GET /api/watch/rooms/{id}", authUser(a.getWatchRoom))
	mux.HandleFunc("POST /api/watch/rooms/{id}/join", authUser(a.joinWatchRoom))
	mux.HandleFunc("PUT /api/watch/rooms/{id}", authUser(a.updateWatchRoom))

	mux.HandleFunc("GET /api/live", authUser(a.listLiveStreams))
	mux.HandleFunc("POST /api/live", authAdmin(a.createLiveStream))
	mux.HandleFunc("GET /api/live/{id}", authUser(a.getLiveStream))
	mux.HandleFunc("POST /api/live/{id}/start", authAdmin(a.startLiveStream))
	mux.HandleFunc("POST /api/live/{id}/stop", authAdmin(a.stopLiveStream))
	mux.HandleFunc("GET /api/live/{id}/hls/{file...}", authUser(a.liveHLS))
	mux.HandleFunc("GET /api/live/{id}/chat", authUser(a.listChatMessages))
	mux.HandleFunc("POST /api/live/{id}/chat", authUser(a.sendChatMessage))

	mux.HandleFunc("GET /api/playlists", authUser(a.listPlaylists))
	mux.HandleFunc("POST /api/playlists", authUser(a.createPlaylist))
	mux.HandleFunc("GET /api/playlists/{id}", authUser(a.getPlaylist))
	mux.HandleFunc("PATCH /api/playlists/{id}", authUser(a.updatePlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}", authUser(a.deletePlaylist))
	mux.HandleFunc("POST /api/playlists/{id}/items", authUser(a.addPlaylistItem))
	mux.HandleFunc("DELETE /api/playlists/{id}/items/{videoId}", authUser(a.removePlaylistItem))

	mux.HandleFunc("GET /api/admin/stats", authAdmin(a.stats))
	mux.HandleFunc("GET /api/admin/jobs", authAdmin(a.jobs))
	mux.HandleFunc("GET /api/admin/system", authAdmin(a.system))
	mux.HandleFunc("POST /api/admin/maintenance/run", authAdmin(a.runMaintenanceNow))
	mux.HandleFunc("GET /api/admin/backups", authAdmin(a.listBackups))
	mux.HandleFunc("GET /api/admin/backups/{name}", authAdmin(a.downloadBackup))
	mux.HandleFunc("GET /api/admin/webhooks", authAdmin(a.listWebhooks))
	mux.HandleFunc("POST /api/admin/webhooks", authAdmin(a.createWebhook))
	mux.HandleFunc("PATCH /api/admin/webhooks/{id}", authAdmin(a.updateWebhook))
	mux.HandleFunc("DELETE /api/admin/webhooks/{id}", authAdmin(a.deleteWebhook))
	mux.HandleFunc("GET /api/openapi.json", func(w http.ResponseWriter, r *http.Request) { a.openAPI(w, r) })
	mux.HandleFunc("GET /api/videos/{id}/skip-intervals", authUser(a.getSkipIntervals))
	mux.HandleFunc("PUT /api/videos/{id}/skip-interval", authUser(a.setSkipInterval))
	mux.HandleFunc("DELETE /api/videos/{id}/skip-interval", authUser(a.clearSkipInterval))
	mux.HandleFunc("GET /api/admin/export", authAdmin(a.exportAll))
	mux.HandleFunc("POST /api/admin/import", authAdmin(a.importBackup))
	mux.HandleFunc("GET /api/admin/paths", authAdmin(a.listServerPaths))
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	webRoot := a.webRoot()
	if webRoot == "" {
		return a.recoverer(a.logger(a.cors(mux)))
	}

	// production mode: serve the built React app from the Go server
	apiMux := mux
	return a.recoverer(a.logger(a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			apiMux.ServeHTTP(w, r)
			return
		}
		serveSPA(w, r, webRoot)
	}))))
}

func (a *App) webRoot() string {
	if a.cfg.WebRoot != "" {
		return a.cfg.WebRoot
	}
	candidates := []string{"frontend/dist", "../frontend/dist", "./frontend/dist"}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}

func serveSPA(w http.ResponseWriter, r *http.Request, root string) {
	cleanRoot := filepath.Clean(root)
	path := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(r.URL.Path)))
	if strings.HasPrefix(path, cleanRoot+string(os.PathSeparator)) {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
	}
	// SPA fallback: unknown paths are client-side routes
	http.ServeFile(w, r, filepath.Join(cleanRoot, "index.html"))
}

func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if len(a.cfg.CORSOrigins) == 0 || origin == "" {
			// No allow-list configured: accept any origin (token auth, no cookies).
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if slices.Contains(a.cfg.CORSOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Range, Content-Length")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v\n%s", rec, debug.Stack())
				writeErr(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
