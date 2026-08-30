package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"videocms/backend/internal/models"
)

var videoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mkv": true, ".webm": true,
	".avi": true, ".mov": true, ".ts": true, ".m2ts": true,
	".flv": true, ".wmv": true, ".mpg": true, ".mpeg": true,
	".3gp": true, ".ogv": true,
}

// subtitleExts in preference order: when several sidecar files exist next to a
// video, the first match wins. Kept as a slice (not a map) so the choice is
// deterministic.
var subtitleExts = []string{".srt", ".vtt", ".ass", ".ssa"}

type Scanner struct {
	pool       *pgxpool.Pool
	dataDir    string
	ffprobeBin string
	ffmpegBin  string
	enricher   MetadataEnricher
	onDone     func(libraryName, status string)
	mu         sync.Mutex
	active     map[uuid.UUID]context.CancelFunc
	watching   bool
}

// MetadataEnricher fills in metadata after a video is indexed (e.g. TMDB).
type MetadataEnricher interface {
	MaybeScrape(ctx context.Context, videoID uuid.UUID) error
}

func (s *Scanner) SetEnricher(e MetadataEnricher) {
	s.enricher = e
}

// SetNotify registers a completion callback (library name + final status).
func (s *Scanner) SetNotify(fn func(libraryName, status string)) {
	s.onDone = fn
}

func NewScanner(pool *pgxpool.Pool, dataDir string) *Scanner {
	return &Scanner{
		pool:       pool,
		dataDir:    dataDir,
		ffprobeBin: ResolveTool("ffprobe"),
		ffmpegBin:  ResolveTool("ffmpeg"),
		active:     make(map[uuid.UUID]context.CancelFunc),
	}
}

// ResolveTool prefers an explicit FFMPEG_BIN/FFPROBE_BIN env var, then the
// Homebrew ffmpeg formula path (which is usually a complete build), then PATH.
func ResolveTool(name string) string {
	envName := strings.ToUpper(name) + "_BIN"
	if v := os.Getenv(envName); v != "" {
		return v
	}
	brew := filepath.Join("/usr/local/opt/ffmpeg/bin", name)
	if st, err := os.Stat(brew); err == nil && !st.IsDir() {
		return brew
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

// Start kicks off a background scan for the library. Returns false if a scan is already running.
func (s *Scanner) Start(ctx context.Context, lib models.Library) bool {
	s.mu.Lock()
	if _, ok := s.active[lib.ID]; ok {
		s.mu.Unlock()
		return false
	}
	scanCtx, cancel := context.WithCancel(context.Background())
	s.active[lib.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("scan panic for library %q: %v\n%s", lib.Name, rec, debug.Stack())
				now := time.Now()
				s.setStatus(context.Background(), lib.ID, "error", "scanner panic: "+fmt.Sprint(rec), nil, &now)
			}
			s.mu.Lock()
			delete(s.active, lib.ID)
			s.mu.Unlock()
			var st string
			_ = s.pool.QueryRow(context.Background(),
				`SELECT scan_status FROM libraries WHERE id=$1`, lib.ID).Scan(&st)
			if s.onDone != nil {
				s.onDone(lib.Name, st)
			}
		}()
		s.scan(scanCtx, lib)
	}()
	return true
}

func (s *Scanner) IsScanning(libID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.active[libID]
	return ok
}

// Cancel stops a running scan. Returns false if the library is not scanning.
func (s *Scanner) Cancel(libID uuid.UUID) bool {
	s.mu.Lock()
	cancel, ok := s.active[libID]
	s.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

type probeInfo struct {
	Duration  float64
	Size      int64
	Width     int
	Height    int
	Codec     string
	Container string
	Subs      []probeSubtitle // text subtitle streams inside the container
}

type probeSubtitle struct {
	Index    int
	Codec    string
	Language string
	Title    string
}

func (s *Scanner) scan(ctx context.Context, lib models.Library) {
	scanStart := time.Now()
	s.setStatus(ctx, lib.ID, "scanning", "", &scanStart, nil)
	log.Printf("scanning library %q at %s", lib.Name, lib.Path)

	workers := scanWorkers()
	paths := make(chan string, workers*4)
	walkErrCh := make(chan error, 1)
	go func() {
		defer close(paths)
		walkErrCh <- walkVideos(lib.Path, func(path string, _ fs.DirEntry) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			select {
			case paths <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}()

	found := s.indexFiles(ctx, lib.ID, paths, workers, func(n int64) {
		// update the library count periodically so the UI shows progress
		if _, err := s.pool.Exec(ctx,
			`UPDATE libraries SET video_count=$1 WHERE id=$2`, n, lib.ID); err != nil {
			log.Printf("update scan progress: %v", err)
		}
	})
	walkErr := <-walkErrCh

	finished := time.Now()
	if ctx.Err() != nil {
		s.setStatus(context.Background(), lib.ID, "cancelled", "scan cancelled", &scanStart, &finished)
		log.Printf("scan cancelled for %q after %d videos", lib.Name, found)
		return
	}

	if walkErr != nil {
		log.Printf("scan %s had errors: %v", lib.Path, walkErr)
	}
	s.markMissing(ctx, lib.ID, scanStart)
	s.rebuildSeries(ctx, lib)

	status := "idle"
	scanErr := ""
	if walkErr != nil {
		status = "error"
		scanErr = walkErr.Error()
	}
	s.setStatus(context.Background(), lib.ID, status, scanErr, &scanStart, &finished)
	if _, err := s.pool.Exec(ctx,
		`UPDATE libraries SET video_count=$1 WHERE id=$2`, found, lib.ID); err != nil {
		log.Printf("update library count: %v", err)
	}
	log.Printf("scan finished for %q: %d videos", lib.Name, found)
}

// walkVideos walks root and calls fn for every video file, applying the same
// skip rules as a full scan: hidden entries, HLS stream folders (xxx.m3u8/),
// and non-video extensions are ignored. Entry errors are logged and skipped so
// one unreadable file does not abort the walk; the first such error is returned
// at the end.
func walkVideos(root string, fn func(path string, d fs.DirEntry) error) error {
	var walkErr error
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("scan: %v", err)
			if walkErr == nil {
				walkErr = err
			}
			return nil
		}
		if d.IsDir() {
			// skip hidden directories (.Trashes, .Spotlight-V100, .git, ...)
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			// skip HLS stream folders (xxx.m3u8/index/N.ts) — not standalone videos
			if strings.HasSuffix(strings.ToLower(d.Name()), ".m3u8") {
				return filepath.SkipDir
			}
			return nil
		}
		// skip AppleDouble resource forks and other hidden files (macOS "._*" copies)
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !videoExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		return fn(path, d)
	})
	if err != nil {
		return err
	}
	return walkErr
}

// indexFiles probes and upserts every path from the channel using a small
// worker pool. found counts the videos successfully indexed. onProgress, when
// set, is called periodically with the running count.
func (s *Scanner) indexFiles(ctx context.Context, libID uuid.UUID, paths chan string, workers int, onProgress func(found int64)) int64 {
	var found atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				if ctx.Err() != nil {
					return
				}
				info, err := s.probe(ctx, path)
				if err != nil {
					log.Printf("probe %s: %v", path, err)
					// index it anyway so the user can see/fix the file; metadata stays empty
					if err2 := s.upsert(ctx, libID, path, probeInfo{}); err2 != nil {
						log.Printf("upsert %s: %v", path, err2)
					}
					continue
				}
				if err := s.upsert(ctx, libID, path, info); err != nil {
					log.Printf("upsert %s: %v", path, err)
					continue
				}
				n := found.Add(1)
				if onProgress != nil && n%20 == 0 {
					onProgress(n)
				}
			}
		}()
	}
	wg.Wait()
	return found.Load()
}

// Watch keeps the library index in sync with the disk. It reacts to
// filesystem events immediately via fsnotify (recursively watched library
// roots), and keeps the proven full diff-based pass running every interval as
// a safety net for missed events. An interval <= 0 disables watching.
func (s *Scanner) Watch(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	log.Printf("filesystem watcher enabled: fsnotify events + fallback scan every %s", interval)
	s.watchOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	if watcher, err := fsnotify.NewWatcher(); err == nil {
		defer func() { _ = watcher.Close() }()
		go s.watchEvents(ctx, watcher)
	} else {
		log.Printf("fsnotify unavailable (%v); falling back to periodic scans", err)
	}
	for {
		select {
		case <-ctx.Done():
			log.Println("filesystem watcher stopped")
			return
		case <-ticker.C:
			s.watchOnce(ctx)
		}
	}
}

type eventDirty struct {
	videos  map[string]struct{}
	removed map[string]struct{}
}

// watchEvents reacts to filesystem events under every library root, collects
// changed paths per library and flushes them after a short quiet period.
func (s *Scanner) watchEvents(ctx context.Context, watcher *fsnotify.Watcher) {
	dirToLib := map[string]uuid.UUID{}
	dirty := map[uuid.UUID]*eventDirty{}
	var timer *time.Timer

	addWatch := func(dir string, libID uuid.UUID) {
		if _, ok := dirToLib[dir]; ok {
			return
		}
		if err := watcher.Add(dir); err == nil {
			dirToLib[dir] = libID
		} else if !os.IsNotExist(err) {
			log.Printf("watcher: cannot watch %s: %v", dir, err)
		}
	}
	libForPath := func(path string) (uuid.UUID, bool) {
		for dir := path; ; dir = filepath.Dir(dir) {
			if id, ok := dirToLib[dir]; ok {
				return id, true
			}
			if dir == filepath.Dir(dir) {
				return uuid.Nil, false
			}
		}
	}
	flush := func() {
		timer = nil
		for libID, d := range dirty {
			if s.IsScanning(libID) {
				continue // a full scan is already indexing this library
			}
			s.applyEventBatch(ctx, libID, d.videos, d.removed)
		}
		dirty = map[uuid.UUID]*eventDirty{}
	}
	schedule := func() {
		if timer == nil {
			timer = time.AfterFunc(2*time.Second, flush)
		}
	}

	libs, err := s.loadLibraries(ctx)
	if err != nil {
		log.Printf("watcher: load libraries for fsnotify: %v", err)
		return
	}
	for _, lib := range libs {
		if st, err := os.Stat(lib.Path); err == nil && st.IsDir() {
			addWatch(lib.Path, lib.ID)
			// fsnotify watches are not recursive: register every existing
			// subdirectory so events in nested folders are seen too.
			_ = filepath.WalkDir(lib.Path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() && p != lib.Path {
					addWatch(p, lib.ID)
				}
				return nil
			})
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: fsnotify error: %v", err)
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			libID, ok := libForPath(ev.Name)
			if !ok {
				continue
			}
			d := dirty[libID]
			if d == nil {
				d = &eventDirty{videos: map[string]struct{}{}, removed: map[string]struct{}{}}
				dirty[libID] = d
			}
			base := filepath.Base(ev.Name)
			if strings.HasPrefix(base, "._") || base == ".DS_Store" {
				continue
			}
			isFile := isVideoFile(base) || isSubtitleFile(base)
			if ev.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				if isFile {
					d.videos[ev.Name] = struct{}{}
				} else if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					addWatch(ev.Name, libID)
				}
				schedule()
				continue
			}
			if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				if isFile {
					d.videos[ev.Name] = struct{}{}
				} else {
					d.removed[ev.Name] = struct{}{}
					for dir := range dirToLib {
						if dir == ev.Name || strings.HasPrefix(dir, ev.Name+string(os.PathSeparator)) {
							delete(dirToLib, dir)
						}
					}
				}
				schedule()
			}
		}
	}
}

func isVideoFile(name string) bool {
	return videoExts[strings.ToLower(filepath.Ext(name))]
}

func isSubtitleFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range subtitleExts {
		if e == ext {
			return true
		}
	}
	return false
}

// applyEventBatch reprobes exactly the files that changed on disk and marks
// removed files/directories unavailable — no full tree walk.
func (s *Scanner) applyEventBatch(ctx context.Context, libID uuid.UUID, videos map[string]struct{}, removedDirs map[string]struct{}) {
	changed := false
	for dir := range removedDirs {
		tag, err := s.pool.Exec(ctx, `
			UPDATE videos SET available=false, updated_at=now()
			WHERE library_id=$1 AND available AND starts_with(file_path, $2 || '/')`,
			libID, dir)
		if err != nil {
			log.Printf("watcher: mark removed dir %s: %v", dir, err)
		} else if tag.RowsAffected() > 0 {
			changed = true
		}
	}

	var candidates []string
	for p := range videos {
		base := filepath.Base(p)
		// subtitle sidecar events must not be probed as videos: resync the
		// sibling video's track list instead.
		if isSubtitleFile(base) {
			videoBase := strings.TrimSuffix(p, filepath.Ext(p))
			var vid uuid.UUID
			var videoPath string
			err := s.pool.QueryRow(ctx, `
				SELECT id, file_path FROM videos
				WHERE file_path LIKE $1 || '.%' ORDER BY length(file_path) LIMIT 1`,
				videoBase).Scan(&vid, &videoPath)
			if err == nil {
				active, err := s.syncSubtitleTracks(ctx, vid, videoPath, findSidecarSubtitles(videoPath), nil)
				if err != nil {
					log.Printf("watcher: resync subtitle tracks: %v", err)
				}
				var current string
				if err := s.pool.QueryRow(ctx,
					`SELECT subtitle_path FROM videos WHERE id=$1`, vid).Scan(&current); err == nil && active != "" && active != current {
					_, _ = s.pool.Exec(ctx,
						`UPDATE videos SET subtitle_path=$1, updated_at=now() WHERE id=$2`, active, vid)
				}
			}
			continue
		}
		if !isVideoFile(base) {
			continue // not media; ignore other events
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			candidates = append(candidates, p)
			continue
		}
		var vid uuid.UUID
		if err := s.pool.QueryRow(ctx,
			`SELECT id FROM videos WHERE file_path=$1`, p).Scan(&vid); err != nil {
			continue // never indexed
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE videos SET available=false, updated_at=now() WHERE id=$1`, vid); err == nil {
			changed = true
		}
		if _, err := s.pool.Exec(ctx,
			`DELETE FROM subtitle_tracks WHERE video_id=$1 AND kind IN ('sidecar','embedded')`, vid); err != nil {
			log.Printf("watcher: drop subtitle tracks: %v", err)
		}
	}

	if len(candidates) > 0 {
		paths := make(chan string, scanWorkers()*4)
		go func() {
			defer close(paths)
			for _, p := range candidates {
				select {
				case paths <- p:
				case <-ctx.Done():
					return
				}
			}
		}()
		if n := s.indexFiles(ctx, libID, paths, scanWorkers(), nil); n > 0 {
			changed = true
		}
	}

	if !changed {
		return
	}
	var lib models.Library
	if err := s.pool.QueryRow(ctx,
		`SELECT id, name, path FROM libraries WHERE id=$1`, libID).
		Scan(&lib.ID, &lib.Name, &lib.Path); err != nil {
		log.Printf("watcher: load library: %v", err)
		return
	}
	s.rebuildSeries(ctx, lib)
	if _, err := s.pool.Exec(ctx,
		`UPDATE libraries SET video_count=(SELECT count(*) FROM videos WHERE library_id=$1 AND available=true) WHERE id=$2`,
		libID, libID); err != nil {
		log.Printf("watcher: update video count: %v", err)
	}
}

// watchOnce runs one incremental pass over every library. Passes never overlap;
// libraries with a full scan in progress are left to the scanner.
func (s *Scanner) watchOnce(ctx context.Context) {
	s.mu.Lock()
	if s.watching {
		s.mu.Unlock()
		return
	}
	s.watching = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.watching = false
		s.mu.Unlock()
	}()

	libs, err := s.loadLibraries(ctx)
	if err != nil {
		log.Printf("watcher: load libraries: %v", err)
		return
	}
	for _, lib := range libs {
		if ctx.Err() != nil {
			return
		}
		if s.IsScanning(lib.ID) {
			continue // a full scan is already indexing this library
		}
		if err := s.watchLibrary(ctx, lib); err != nil {
			log.Printf("watcher: library %q: %v", lib.Name, err)
		}
	}
}

func (s *Scanner) loadLibraries(ctx context.Context) ([]models.Library, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, path FROM libraries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	libs := []models.Library{}
	for rows.Next() {
		var lib models.Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.Path); err != nil {
			return nil, err
		}
		libs = append(libs, lib)
	}
	return libs, rows.Err()
}

type watchedRow struct {
	id           uuid.UUID
	sizeBytes    int64
	available    bool
	subtitlePath string
}

type subtitleUpdate struct {
	id   uuid.UUID
	path string
}

// watchLibrary runs one incremental pass over a single library: only new or
// changed files are probed, files that reappear are re-enabled, and files that
// disappeared are marked unavailable. Series grouping is rebuilt whenever
// something changed.
func (s *Scanner) watchLibrary(ctx context.Context, lib models.Library) error {
	if st, err := os.Stat(lib.Path); err != nil || !st.IsDir() {
		// A temporarily unavailable mount should not hide the whole library.
		log.Printf("watcher: library %q path not available: %v", lib.Name, lib.Path)
		return nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, file_path, size_bytes, available, subtitle_path FROM videos WHERE library_id=$1`, lib.ID)
	if err != nil {
		return fmt.Errorf("load indexed videos: %w", err)
	}
	existing := map[string]watchedRow{}
	for rows.Next() {
		var r watchedRow
		var path string
		if err := rows.Scan(&r.id, &path, &r.sizeBytes, &r.available, &r.subtitlePath); err != nil {
			rows.Close()
			return fmt.Errorf("read indexed video: %w", err)
		}
		existing[path] = r
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	passStart := time.Now()
	var changed bool
	var candidates []string
	var revived []uuid.UUID
	var touchIDs []uuid.UUID
	var subtitleUpdates []subtitleUpdate

	if err := walkVideos(lib.Path, func(path string, d fs.DirEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r, ok := existing[path]
		if !ok {
			changed = true
			candidates = append(candidates, path)
			return nil
		}
		if !r.available {
			changed = true
			revived = append(revived, r.id)
			return nil
		}
		if info, err := d.Info(); err == nil && info.Size() != r.sizeBytes {
			changed = true
			candidates = append(candidates, path)
			return nil
		}
		if sub := findSubtitle(path); sub != r.subtitlePath {
			changed = true
			subtitleUpdates = append(subtitleUpdates, subtitleUpdate{id: r.id, path: sub})
		}
		touchIDs = append(touchIDs, r.id)
		return nil
	}); err != nil && ctx.Err() == nil {
		log.Printf("watcher: walk %s: %v", lib.Path, err)
	}
	if ctx.Err() != nil {
		return nil
	}

	if len(candidates) > 0 {
		paths := make(chan string, scanWorkers()*4)
		go func() {
			defer close(paths)
			for _, p := range candidates {
				select {
				case paths <- p:
				case <-ctx.Done():
					return
				}
			}
		}()
		s.indexFiles(ctx, lib.ID, paths, scanWorkers(), nil)
	}
	if ctx.Err() != nil {
		return nil
	}

	if len(touchIDs) > 0 {
		if _, err := s.pool.Exec(ctx,
			`UPDATE videos SET last_scanned_at=now() WHERE id = ANY($1::uuid[]) AND available=true`, touchIDs); err != nil {
			log.Printf("watcher: refresh scanned-at: %v", err)
		}
	}
	for _, id := range revived {
		if _, err := s.pool.Exec(ctx,
			`UPDATE videos SET available=true, updated_at=now(), last_scanned_at=now() WHERE id=$1`, id); err != nil {
			log.Printf("watcher: revive video: %v", err)
		}
	}
	for _, u := range subtitleUpdates {
		if _, err := s.pool.Exec(ctx,
			`UPDATE videos SET subtitle_path=$1, updated_at=now(), last_scanned_at=now() WHERE id=$2`, u.path, u.id); err != nil {
			log.Printf("watcher: update subtitle: %v", err)
		}
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE videos SET available=false, updated_at=now()
		 WHERE library_id=$1 AND available=true AND last_scanned_at < $2`,
		lib.ID, passStart)
	if err != nil {
		log.Printf("watcher: mark missing: %v", err)
	} else if tag.RowsAffected() > 0 {
		changed = true
	}

	if changed {
		s.rebuildSeries(ctx, lib)
		if _, err := s.pool.Exec(ctx,
			`UPDATE libraries SET video_count=(SELECT count(*) FROM videos WHERE library_id=$1 AND available=true) WHERE id=$2`,
			lib.ID, lib.ID); err != nil {
			log.Printf("watcher: update video count: %v", err)
		}
	}
	return nil
}

// rebuildSeries groups available videos in a library into TV series (>=2
// episodes per group), sorted by season/episode. Videos with episode markers
// in their filename are grouped by the parsed series name; videos whose
// filename is a bare episode number fall back to the containing directory
// (or the library name) as the series name.
func (s *Scanner) rebuildSeries(ctx context.Context, lib models.Library) {
	type candidate struct {
		videoID uuid.UUID
		season  int
		episode int
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, title, file_path FROM videos WHERE library_id=$1 AND available=true`, lib.ID)
	if err != nil {
		log.Printf("rebuild series query: %v", err)
		return
	}
	byKey := map[string][]candidate{}
	nameOf := map[string]string{}
	for rows.Next() {
		var videoID uuid.UUID
		var title, filePath string
		if err := rows.Scan(&videoID, &title, &filePath); err != nil {
			continue
		}
		seriesName, season, episode := parseEpisode(title)
		if seriesName == "" || episode == 0 {
			if m := bareNumberRe.FindStringSubmatch(title); m != nil {
				ep, _ := strconv.Atoi(m[1])
				if ep > 0 {
					seriesName = fallbackSeriesName(lib.Name, lib.Path, filePath)
					season = 0
					episode = ep
				}
			}
		}
		if seriesName == "" || episode == 0 {
			continue
		}
		key := strings.ToLower(seriesName) + "\x00" + strconv.Itoa(season)
		byKey[key] = append(byKey[key], candidate{videoID: videoID, season: season, episode: episode})
		if _, ok := nameOf[key]; !ok {
			nameOf[key] = seriesName
		}
	}
	rows.Close()

	// reset previous assignments for this library (available rows)
	if _, err := s.pool.Exec(ctx,
		`UPDATE videos SET series_id=NULL, season=0, episode=0
		 WHERE library_id=$1 AND available=true`, lib.ID); err != nil {
		log.Printf("reset series assignments: %v", err)
		return
	}

	for key, group := range byKey {
		if len(group) < 2 {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		season, _ := strconv.Atoi(parts[1])
		var seriesID uuid.UUID
		err := s.pool.QueryRow(ctx, `
			INSERT INTO series (library_id, name, season, episode_count)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (library_id, name, season) DO UPDATE SET
				episode_count=EXCLUDED.episode_count, updated_at=now()
			RETURNING id`,
			lib.ID, nameOf[key], season, len(group)).Scan(&seriesID)
		if err != nil {
			log.Printf("upsert series %q: %v", nameOf[key], err)
			continue
		}
		for _, c := range group {
			if _, err := s.pool.Exec(ctx,
				`UPDATE videos SET series_id=$1, season=$2, episode=$3 WHERE id=$4`,
				seriesID, c.season, c.episode, c.videoID); err != nil {
				log.Printf("assign episode: %v", err)
			}
		}
	}

	// remove series that no longer have at least 2 available episodes
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM series WHERE library_id=$1 AND (
			SELECT count(*) FROM videos v WHERE v.series_id=series.id AND v.available
		) < 2`, lib.ID); err != nil {
		log.Printf("cleanup empty series: %v", err)
	}
	log.Printf("series rebuilt for library %s: %d groups", lib.ID.String()[:8], len(byKey))
}

func scanWorkers() int {
	n := 4
	if v := os.Getenv("SCAN_WORKERS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 16 {
			n = parsed
		}
	}
	return n
}

func (s *Scanner) setStatus(ctx context.Context, libID uuid.UUID, status, scanErr string, started, finished *time.Time) {
	_, err := s.pool.Exec(ctx,
		`UPDATE libraries SET scan_status=$1, scan_error=$2, scan_started_at=$3, scan_finished_at=$4 WHERE id=$5`,
		status, scanErr, started, finished, libID)
	if err != nil {
		log.Printf("set scan status: %v", err)
	}
}

func (s *Scanner) upsert(ctx context.Context, libID uuid.UUID, path string, info probeInfo) error {
	filename := filepath.Base(path)
	title, year := deriveTitle(filename)
	sidecars := findSidecarSubtitles(path)
	subtitle := ""
	if len(sidecars) > 0 {
		subtitle = sidecars[0]
	}

	var id uuid.UUID
	var posterPath string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO videos (
			library_id, title, filename, file_path, size_bytes, duration_sec,
			width, height, video_codec, container, year, subtitle_path,
			available, updated_at, last_scanned_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,true,now(),now())
		ON CONFLICT (file_path) DO UPDATE SET
			library_id = EXCLUDED.library_id,
			title = EXCLUDED.title,
			filename = EXCLUDED.filename,
			size_bytes = EXCLUDED.size_bytes,
			duration_sec = EXCLUDED.duration_sec,
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			video_codec = EXCLUDED.video_codec,
			container = EXCLUDED.container,
			year = EXCLUDED.year,
			subtitle_path = COALESCE(NULLIF(EXCLUDED.subtitle_path, ''), videos.subtitle_path),
			available = true,
			updated_at = now(),
			last_scanned_at = now()
		RETURNING id, poster_path`,
		libID, title, filename, path, info.Size, info.Duration,
		info.Width, info.Height, info.Codec, info.Container, year, subtitle,
	).Scan(&id, &posterPath)
	if err != nil {
		return fmt.Errorf("upsert video: %w", err)
	}

	if posterPath == "" {
		if p, err := s.extractPoster(ctx, id, path, info.Duration); err == nil && p != "" {
			if _, err := s.pool.Exec(ctx,
				`UPDATE videos SET poster_path=$1 WHERE id=$2`, p, id); err != nil {
				log.Printf("save poster path: %v", err)
			}
		} else if err != nil {
			log.Printf("poster extraction for %s: %v", path, err)
		}
	}
	active, err := s.syncSubtitleTracks(ctx, id, path, sidecars, info.Subs)
	if err != nil {
		log.Printf("sync subtitle tracks for %s: %v", path, err)
	}
	if active != "" && active != subtitle {
		if _, err := s.pool.Exec(ctx,
			`UPDATE videos SET subtitle_path=$1 WHERE id=$2`, active, id); err != nil {
			log.Printf("save active subtitle path: %v", err)
		}
	}
	if s.enricher != nil {
		if err := s.enricher.MaybeScrape(ctx, id); err != nil {
			log.Printf("enrich %s: %v", path, err)
		}
	}
	return nil
}

func (s *Scanner) markMissing(ctx context.Context, libID uuid.UUID, scanStart time.Time) {
	if _, err := s.pool.Exec(ctx,
		`UPDATE videos SET available=false, updated_at=now()
		 WHERE library_id=$1 AND available=true AND last_scanned_at < $2`,
		libID, scanStart); err != nil {
		log.Printf("mark missing: %v", err)
	}
}

func (s *Scanner) probe(ctx context.Context, path string) (probeInfo, error) {
	var info probeInfo
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ffprobeBin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
	out, err := cmd.Output()
	if err != nil {
		return info, fmt.Errorf("ffprobe: %w", err)
	}

	var raw struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Index     int    `json:"index"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
		Format struct {
			Duration   string `json:"duration"`
			Size       string `json:"size"`
			FormatName string `json:"format_name"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return info, fmt.Errorf("parse ffprobe output: %w", err)
	}
	info.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	info.Size, _ = strconv.ParseInt(raw.Format.Size, 10, 64)
	info.Container = raw.Format.FormatName
	for _, st := range raw.Streams {
		switch st.CodecType {
		case "video":
			info.Codec = st.CodecName
			info.Width = st.Width
			info.Height = st.Height
		case "subtitle":
			if IsImageSubtitleCodec(st.CodecName) {
				continue
			}
			info.Subs = append(info.Subs, probeSubtitle{
				Index:    st.Index,
				Codec:    st.CodecName,
				Language: st.Tags.Language,
				Title:    st.Tags.Title,
			})
		}
	}
	return info, nil
}

// extractEmbedded extracts one embedded text subtitle stream into the server's
// subtitle directory and returns the stored file path.
func (s *Scanner) extractEmbedded(ctx context.Context, videoID uuid.UUID, path string, streamIndex int) (string, error) {
	dir := filepath.Join(s.dataDir, "subtitles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fmt.Sprintf("%s-e%d.vtt", videoID.String(), streamIndex))
	if err := ExtractEmbeddedSubtitle(ctx, s.ffmpegBin, path, streamIndex, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *Scanner) extractPoster(ctx context.Context, videoID uuid.UUID, path string, duration float64) (string, error) {
	posterDir := filepath.Join(s.dataDir, "posters")
	if err := os.MkdirAll(posterDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(posterDir, videoID.String()+".jpg")
	ss := math.Max(0, duration*0.15)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpegBin, "-y", "-ss", fmt.Sprintf("%.2f", ss),
		"-i", path, "-frames:v", "1", "-vf", "scale=480:-2", "-q:v", "4", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg: %w: %s", err, truncate(string(out), 300))
	}
	return dst, nil
}

func findSubtitle(videoPath string) string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	for _, ext := range subtitleExts {
		p := base + ext
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// findSidecarSubtitles returns every existing sidecar subtitle file for a
// video, in preference order. Unlike findSubtitle it returns all of them so
// the subtitle-track UI can offer multi-language sidecars.
func findSidecarSubtitles(videoPath string) []string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	var out []string
	for _, ext := range subtitleExts {
		p := base + ext
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// syncSubtitleTracks reconciles the subtitle_tracks rows for a video with the
// sidecar files and embedded text streams currently on disk. Uploaded tracks
// are never touched. It returns the path that should be active (first sidecar,
// else the first embedded track, else ""), extracting embedded tracks on
// demand as needed.
func (s *Scanner) syncSubtitleTracks(ctx context.Context, videoID uuid.UUID, videoPath string, sidecars []string, embeds []probeSubtitle) (string, error) {
	wanted := map[string]struct{}{}
	insert := func(kind, sourceKey, path, lang, title string, pos, streamIndex int) error {
		wanted[sourceKey] = struct{}{}
		_, err := s.pool.Exec(ctx, `
			INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key, stream_index)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (video_id, source_key) WHERE source_key <> ''
			DO UPDATE SET position=EXCLUDED.position, lang=EXCLUDED.lang,
			              title=EXCLUDED.title, path=EXCLUDED.path,
			              stream_index=EXCLUDED.stream_index`,
			videoID, pos, lang, title, path, kind, sourceKey, streamIndex)
		return err
	}

	for i, p := range sidecars {
		if err := insert("sidecar", "sidecar:"+filepath.Base(p), p, "", "", i, -1); err != nil {
			return "", err
		}
	}
	for i, e := range embeds {
		if err := insert("embedded", "embedded:"+strconv.Itoa(e.Index), "", e.Language, e.Title, len(sidecars)+i, e.Index); err != nil {
			return "", err
		}
	}

	keys := make([]string, 0, len(wanted))
	for k := range wanted {
		keys = append(keys, k)
	}
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM subtitle_tracks
		WHERE video_id=$1 AND kind IN ('sidecar','embedded')
		  AND (source_key = '' OR NOT (source_key = ANY($2::text[])))`,
		videoID, keys); err != nil {
		return "", err
	}

	if len(sidecars) > 0 {
		return sidecars[0], nil
	}
	if len(embeds) > 0 {
		dst, err := s.extractEmbedded(ctx, videoID, videoPath, embeds[0].Index)
		if err != nil {
			log.Printf("extract embedded subtitle for %s: %v", videoPath, err)
			return "", nil
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE subtitle_tracks SET path=$1 WHERE video_id=$2 AND source_key=$3`,
			dst, videoID, "embedded:"+strconv.Itoa(embeds[0].Index)); err != nil {
			return "", err
		}
		return dst, nil
	}
	return "", nil
}

var (
	yearRe  = regexp.MustCompile(`[\(\[]\s*(\d{4})\s*[\)\]]`)
	tagRe   = regexp.MustCompile(`(?i)(\b\d{3,4}p\b|\b\d+k\b|\bx264\b|\bx265\b|\bhevc\b|\bh\.?264\b|\bh\.?265\b|\bweb-?dl\b|\bwebrip\b|\bbluray\b|\bhdtv\b|\bremux\b|\bhdrip\b|\bbdrip\b|\bamzn?\b|\bnetflix\b|\bdisney\+?\b)`)
	spaceRe = regexp.MustCompile(`[\s._\-]+`)
)

func deriveTitle(filename string) (string, int) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	year := 0
	if m := yearRe.FindStringSubmatch(base); m != nil {
		year, _ = strconv.Atoi(m[1])
		base = yearRe.ReplaceAllString(base, " ")
	}
	base = tagRe.ReplaceAllString(base, " ")
	base = spaceRe.ReplaceAllString(strings.TrimSpace(base), " ")
	title := strings.TrimSpace(base)
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	return title, year
}

func truncate(s string, n int) string {
	// keep the tail of the output: ffmpeg error details are at the end, the
	// version banner at the start is useless noise
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
