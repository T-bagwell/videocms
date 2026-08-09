package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tmdbMinInterval = 400 * time.Millisecond

type Scraper struct {
	pool    *pgxpool.Pool
	dataDir string
	apiKey  string
	lang    string
	client  *http.Client
	mu      sync.Mutex
	last    time.Time
}

func NewScraper(pool *pgxpool.Pool, dataDir, apiKey string) *Scraper {
	lang := os.Getenv("TMDB_LANGUAGE")
	if lang == "" {
		lang = "zh-CN"
	}
	return &Scraper{
		pool:    pool,
		dataDir: dataDir,
		apiKey:  apiKey,
		lang:    lang,
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *Scraper) Enabled() bool {
	return s.apiKey != ""
}

// MaybeScrape is called during scanning. It only scrapes videos that have no
// synopsis yet, were never scraped, and honors a rate limit.
func (s *Scraper) MaybeScrape(ctx context.Context, videoID uuid.UUID) error {
	if !s.Enabled() {
		return nil
	}
	var title string
	var year int
	var synopsis string
	err := s.pool.QueryRow(ctx,
		`SELECT title, year, synopsis FROM videos WHERE id=$1`, videoID).
		Scan(&title, &year, &synopsis)
	if err != nil {
		return err
	}
	if synopsis != "" {
		return nil
	}
	if err := s.rateLimit(ctx); err != nil {
		return err
	}
	info, err := s.search(ctx, title, year)
	if err != nil {
		return err
	}
	if info.TmdbID == 0 {
		return nil
	}
	return s.apply(ctx, videoID, info)
}

// Scrape performs a manual scrape and always overwrites the existing metadata.
func (s *Scraper) Scrape(ctx context.Context, videoID uuid.UUID) error {
	if !s.Enabled() {
		return errors.New("TMDB_API_KEY is not configured")
	}
	var title string
	var year int
	if err := s.pool.QueryRow(ctx,
		`SELECT title, year FROM videos WHERE id=$1`, videoID).
		Scan(&title, &year); err != nil {
		return errors.New("video not found")
	}
	if err := s.rateLimit(ctx); err != nil {
		return err
	}
	info, err := s.search(ctx, title, year)
	if err != nil {
		return err
	}
	if info.TmdbID == 0 {
		return fmt.Errorf("no TMDB match found for %q", title)
	}
	return s.apply(ctx, videoID, info)
}

type tmdbInfo struct {
	TmdbID   int
	Title    string
	Year     int
	Synopsis string
	Genres   []string
	Poster   string
}

func (s *Scraper) search(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	u := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?api_key=%s&query=%s&language=%s",
		s.apiKey, urlQueryEscape(title), s.lang)
	if year > 0 {
		u += "&year=" + strconv.Itoa(year)
	}

	var resp struct {
		Results []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			ReleaseDate string `json:"release_date"`
			Overview    string `json:"overview"`
			PosterPath  string `json:"poster_path"`
		} `json:"results"`
	}
	if err := s.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return &tmdbInfo{}, nil
	}
	best := resp.Results[0]

	info := &tmdbInfo{
		TmdbID:   best.ID,
		Title:    best.Title,
		Year:     yearFromDate(best.ReleaseDate),
		Synopsis: best.Overview,
		Poster:   best.PosterPath,
	}

	// details for localized genre names
	var detail struct {
		Genres []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	du := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=%s",
		best.ID, s.apiKey, s.lang)
	if err := s.getJSON(ctx, du, &detail); err == nil {
		for _, g := range detail.Genres {
			info.Genres = append(info.Genres, g.Name)
		}
	}
	return info, nil
}

func (s *Scraper) apply(ctx context.Context, videoID uuid.UUID, info *tmdbInfo) error {
	posterPath := ""
	if info.Poster != "" {
		if p, err := s.downloadPoster(ctx, videoID, info.Poster); err == nil {
			posterPath = p
		} else {
			log.Printf("[scrape:%s] poster download failed: %v", videoID.String()[:8], err)
		}
	}
	var oldPoster string
	err := s.pool.QueryRow(ctx, `SELECT poster_path FROM videos WHERE id=$1`, videoID).Scan(&oldPoster)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE videos SET title=$1, year=$2, synopsis=$3, genres=$4, poster_path=$5,
			tmdb_id=$6, scraped_at=now(), updated_at=now() WHERE id=$7`,
		info.Title, info.Year, info.Synopsis, info.Genres, posterPath, info.TmdbID, videoID); err != nil {
		return err
	}
	if oldPoster != "" && oldPoster != posterPath &&
		filepath.Dir(oldPoster) == filepath.Join(s.dataDir, "posters") {
		os.Remove(oldPoster)
	}
	return nil
}

func (s *Scraper) downloadPoster(ctx context.Context, videoID uuid.UUID, path string) (string, error) {
	u := "https://image.tmdb.org/t/p/w500" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("poster http %d", resp.StatusCode)
	}

	head := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, head)
	ct := http.DetectContentType(head[:n])
	ext := ".jpg"
	switch ct {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	}
	dir := filepath.Join(s.dataDir, "posters")
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, videoID.String()+ext)
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := out.Write(head[:n]); err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *Scraper) getJSON(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tmdb http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (s *Scraper) rateLimit(ctx context.Context) error {
	s.mu.Lock()
	wait := tmdbMinInterval - time.Since(s.last)
	s.mu.Unlock()
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	s.last = time.Now()
	s.mu.Unlock()
	return nil
}

func yearFromDate(date string) int {
	if len(date) >= 4 {
		if y, err := strconv.Atoi(date[:4]); err == nil {
			return y
		}
	}
	return 0
}

func urlQueryEscape(s string) string {
	// keep it simple: percent-encode everything except safe chars
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~':
			b.WriteRune(r)
		default:
			b.WriteString(fmt.Sprintf("%%%02X", r))
		}
	}
	return b.String()
}
