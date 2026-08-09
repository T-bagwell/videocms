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
	"time"

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

var subtitleExts = map[string]bool{
	".srt": true, ".vtt": true, ".ass": true, ".ssa": true,
}

type Scanner struct {
	pool       *pgxpool.Pool
	dataDir    string
	ffprobeBin string
	ffmpegBin  string
	enricher   MetadataEnricher
	mu         sync.Mutex
	active     map[uuid.UUID]context.CancelFunc
}

// MetadataEnricher fills in metadata after a video is indexed (e.g. TMDB).
type MetadataEnricher interface {
	MaybeScrape(ctx context.Context, videoID uuid.UUID) error
}

func (s *Scanner) SetEnricher(e MetadataEnricher) {
	s.enricher = e
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
}

func (s *Scanner) scan(ctx context.Context, lib models.Library) {
	scanStart := time.Now()
	s.setStatus(ctx, lib.ID, "scanning", "", &scanStart, nil)
	log.Printf("scanning library %q at %s", lib.Name, lib.Path)

	workers := scanWorkers()
	paths := make(chan string, workers*4)
	var foundMu sync.Mutex
	var found int64
	var processed int64

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
					if err2 := s.upsert(ctx, lib.ID, path, probeInfo{}); err2 != nil {
						log.Printf("upsert %s: %v", path, err2)
					}
					continue
				}
				if err := s.upsert(ctx, lib.ID, path, info); err != nil {
					log.Printf("upsert %s: %v", path, err)
					continue
				}
				foundMu.Lock()
				found++
				processed++
				// update the library count periodically so the UI shows progress
				if found%20 == 0 {
					if _, err := s.pool.Exec(ctx,
						`UPDATE libraries SET video_count=$1 WHERE id=$2`, found, lib.ID); err != nil {
						log.Printf("update scan progress: %v", err)
					}
				}
				foundMu.Unlock()
			}
		}()
	}

	var walkErr error
	err := filepath.WalkDir(lib.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("scan: %v", err)
			walkErr = err
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
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}
		select {
		case paths <- path:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	close(paths)
	wg.Wait()

	finished := time.Now()
	if ctx.Err() != nil {
		s.setStatus(context.Background(), lib.ID, "cancelled", "scan cancelled", &scanStart, &finished)
		log.Printf("scan cancelled for %q after %d videos", lib.Name, found)
		return
	}

	if err != nil || walkErr != nil {
		log.Printf("scan %s had errors: %v", lib.Path, err)
	}
	s.markMissing(ctx, lib.ID, scanStart)
	s.rebuildSeries(ctx, lib.ID)

	status := "idle"
	scanErr := ""
	if err != nil || walkErr != nil {
		status = "error"
		if err != nil {
			scanErr = err.Error()
		} else if walkErr != nil {
			scanErr = walkErr.Error()
		}
	}
	s.setStatus(context.Background(), lib.ID, status, scanErr, &scanStart, &finished)
	if _, err := s.pool.Exec(ctx,
		`UPDATE libraries SET video_count=$1 WHERE id=$2`, found, lib.ID); err != nil {
		log.Printf("update library count: %v", err)
	}
	log.Printf("scan finished for %q: %d videos (%d processed)", lib.Name, found, processed)
}

// rebuildSeries groups available videos in a library that carry episode markers
// into TV series (>=2 episodes per group), sorted by season/episode.
func (s *Scanner) rebuildSeries(ctx context.Context, libID uuid.UUID) {
	type candidate struct {
		videoID uuid.UUID
		season  int
		episode int
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, title FROM videos WHERE library_id=$1 AND available=true`, libID)
	if err != nil {
		log.Printf("rebuild series query: %v", err)
		return
	}
	byKey := map[string][]candidate{}
	nameOf := map[string]string{}
	for rows.Next() {
		var videoID uuid.UUID
		var title string
		if err := rows.Scan(&videoID, &title); err != nil {
			continue
		}
		seriesName, season, episode := parseEpisode(title)
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
		 WHERE library_id=$1 AND available=true`, libID); err != nil {
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
			libID, nameOf[key], season, len(group)).Scan(&seriesID)
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
		) < 2`, libID); err != nil {
		log.Printf("cleanup empty series: %v", err)
	}
	log.Printf("series rebuilt for library %s: %d groups", libID.String()[:8], len(byKey))
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
	subtitle := findSubtitle(path)

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
			subtitle_path = EXCLUDED.subtitle_path,
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
		if st.CodecType == "video" {
			info.Codec = st.CodecName
			info.Width = st.Width
			info.Height = st.Height
			break
		}
	}
	return info, nil
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
	for ext := range subtitleExts {
		p := base + ext
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

var (
	yearRe   = regexp.MustCompile(`[\(\[]\s*(\d{4})\s*[\)\]]`)
	tagRe    = regexp.MustCompile(`(?i)(\b\d{3,4}p\b|\b\d+k\b|\bx264\b|\bx265\b|\bhevc\b|\bh\.?264\b|\bh\.?265\b|\bweb-?dl\b|\bwebrip\b|\bbluray\b|\bhdtv\b|\bremux\b|\bhdrip\b|\bbdrip\b|\bamzn?\b|\bnetflix\b|\bdisney\+?\b)`)
	spaceRe  = regexp.MustCompile(`[\s._\-]+`)
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
