package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/config"
	"videocms/backend/internal/media"
)

type App struct {
	cfg     config.Config
	pool    *pgxpool.Pool
	scanner *media.Scanner
	hls     *media.HLSManager
	scraper *media.Scraper
}

func New(cfg config.Config, pool *pgxpool.Pool) (*App, error) {
	// A server restart leaves any in-flight scan marked "scanning"; reset it.
	if _, err := pool.Exec(context.Background(),
		`UPDATE libraries SET scan_status='error', scan_error='Server restarted, scan interrupted; please rescan'
		 WHERE scan_status='scanning'`); err != nil {
		return nil, err
	}
	scanner := media.NewScanner(pool, cfg.DataDir)
	app := &App{
		cfg:     cfg,
		pool:    pool,
		scanner: scanner,
		hls:     media.NewHLSManager(cfg.DataDir, media.ResolveTool("ffmpeg")),
		scraper: media.NewScraper(pool, cfg.DataDir, cfg.TMDBAPIKey),
	}
	scanner.SetEnricher(app.scraper)
	return app, nil
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
	mux.HandleFunc("GET /api/auth/me", authUser(a.me))

	mux.HandleFunc("GET /api/libraries", authUser(a.listLibraries))
	mux.HandleFunc("POST /api/libraries", authAdmin(a.createLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", authAdmin(a.scanLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan/cancel", authAdmin(a.cancelScan))
	mux.HandleFunc("POST /api/libraries/{id}/open", authAdmin(a.openLibrary))
	mux.HandleFunc("PATCH /api/libraries/{id}", authAdmin(a.setLibraryBlocked))
	mux.HandleFunc("DELETE /api/libraries/{id}", authAdmin(a.deleteLibrary))

	mux.HandleFunc("GET /api/admin/blocked-titles", authAdmin(a.listBlockedTitles))
	mux.HandleFunc("POST /api/admin/blocked-titles", authAdmin(a.createBlockedTitle))
	mux.HandleFunc("DELETE /api/admin/blocked-titles/{id}", authAdmin(a.deleteBlockedTitle))

	mux.HandleFunc("GET /api/videos", authUser(a.listVideos))
	mux.HandleFunc("GET /api/videos/{id}", authUser(a.getVideo))
	mux.HandleFunc("PATCH /api/videos/{id}", authAdmin(a.updateVideo))
	mux.HandleFunc("POST /api/videos/{id}/poster", authAdmin(a.uploadPoster))
	mux.HandleFunc("GET /api/videos/{id}/stream", authUser(a.streamVideo))
	mux.HandleFunc("GET /api/videos/{id}/download", authUser(a.downloadVideo))
	mux.HandleFunc("GET /api/videos/{id}/subtitles", authUser(a.subtitles))
	mux.HandleFunc("GET /api/videos/{id}/subtitle-tracks", authUser(a.listSubtitleTracks))
	mux.HandleFunc("GET /api/videos/{id}/subtitles/{trackId}", authUser(a.getSubtitleTrack))
	mux.HandleFunc("PUT /api/videos/{id}/subtitles/{trackId}/active", authUser(a.setActiveSubtitleTrack))
	mux.HandleFunc("POST /api/videos/{id}/subtitles", authAdmin(a.uploadSubtitle))
	mux.HandleFunc("DELETE /api/videos/{id}/subtitles", authAdmin(a.deleteSubtitle))
	mux.HandleFunc("POST /api/videos/{id}/subtitles/extract", authAdmin(a.extractEmbeddedSubtitle))
	mux.HandleFunc("GET /api/videos/{id}/poster", authUser(a.servePoster))
	mux.HandleFunc("GET /api/videos/{id}/hls/{file...}", authUser(a.hlsHandler))
	mux.HandleFunc("POST /api/videos/{id}/scrape", authAdmin(a.scrapeVideo))
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
	mux.HandleFunc("GET /api/users/me/export", authUser(a.exportMe))

	mux.HandleFunc("GET /api/playlists", authUser(a.listPlaylists))
	mux.HandleFunc("POST /api/playlists", authUser(a.createPlaylist))
	mux.HandleFunc("GET /api/playlists/{id}", authUser(a.getPlaylist))
	mux.HandleFunc("PATCH /api/playlists/{id}", authUser(a.updatePlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}", authUser(a.deletePlaylist))
	mux.HandleFunc("POST /api/playlists/{id}/items", authUser(a.addPlaylistItem))
	mux.HandleFunc("DELETE /api/playlists/{id}/items/{videoId}", authUser(a.removePlaylistItem))

	mux.HandleFunc("GET /api/admin/stats", authAdmin(a.stats))
	mux.HandleFunc("GET /api/admin/export", authAdmin(a.exportAll))
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range")
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
