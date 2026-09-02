package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tmdbMinInterval = 400 * time.Millisecond

type Scraper struct {
	pool      *pgxpool.Pool
	dataDir   string
	apiKey    string
	omdbKey   string
	fanartKey string
	lang      string
	customURL string
	client    *http.Client
	mu        sync.Mutex
	last      time.Time
}

func NewScraper(pool *pgxpool.Pool, dataDir, apiKey string, customURL ...string) *Scraper {
	custom := ""
	if len(customURL) > 0 {
		custom = customURL[0]
	}
	lang := os.Getenv("TMDB_LANGUAGE")
	if lang == "" {
		lang = "zh-CN"
	}
	client := &http.Client{Timeout: 20 * time.Second}
	// TVMaze requires a descriptive User-Agent.
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", "VideoCMS/1.0 (self-hosted media server; https://github.com/T-bagwell/videocms)")
		return http.DefaultTransport.RoundTrip(req)
	})
	return &Scraper{
		pool:      pool,
		dataDir:   dataDir,
		apiKey:    apiKey,
		lang:      lang,
		customURL: custom,
		client:    client,
	}
}

func (s *Scraper) Enabled() bool {
	// TMDB needs a key; without one we fall back to the keyless TVMaze, AniList
	// and Wikipedia APIs (disable individually with the *_ENABLED vars).
	return s.apiKey != "" || s.omdbKey != "" || tvmazeEnabled() || anilistEnabled() || wikipediaEnabled()
}

// SetOMDbKey configures the OMDb provider.
func (s *Scraper) SetOMDbKey(key string) {
	s.omdbKey = key
}

// SetFanartKey configures Fanart.tv artwork enrichment.
func (s *Scraper) SetFanartKey(key string) {
	s.fanartKey = key
}

func tvmazeEnabled() bool {
	return os.Getenv("TVMAZE_ENABLED") != "0"
}

func anilistEnabled() bool {
	return os.Getenv("ANILIST_ENABLED") != "0"
}

func wikipediaEnabled() bool {
	return os.Getenv("WIKIPEDIA_ENABLED") != "0"
}

func wikipediaLang() string {
	if l := os.Getenv("WIKIPEDIA_LANG"); l != "" {
		return l
	}
	return "en"
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
	if info.Title == "" {
		return nil
	}
	return s.apply(ctx, videoID, info)
}

// Scrape performs a manual scrape and always overwrites the existing metadata.
func (s *Scraper) Scrape(ctx context.Context, videoID uuid.UUID) error {
	return s.ScrapeWith(ctx, videoID, "", false)
}

// ScrapeWith enriches a video using a named provider ("custom" or the default
// TMDB/TVMaze chain). force=true overwrites existing metadata.
func (s *Scraper) ScrapeWith(ctx context.Context, videoID uuid.UUID, provider string, force bool) error {
	var title string
	var year int
	if err := s.pool.QueryRow(ctx,
		`SELECT title, year FROM videos WHERE id=$1`, videoID).
		Scan(&title, &year); err != nil {
		return errors.New("video not found")
	}
	if !force {
		var synopsis string
		if err := s.pool.QueryRow(ctx,
			`SELECT synopsis FROM videos WHERE id=$1`, videoID).Scan(&synopsis); err == nil && synopsis != "" {
			return errors.New("video already has metadata; use force=1 to overwrite")
		}
	}
	if strings.EqualFold(provider, "custom") {
		if s.customURL == "" {
			return errors.New("custom scraper not configured (set SCRAPE_CUSTOM_URL with a %s placeholder)")
		}
		info, err := s.searchCustom(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	}
	if strings.EqualFold(provider, "omdb") {
		if s.omdbKey == "" {
			return errors.New("omdb provider not configured (set OMDB_API_KEY)")
		}
		info, err := s.searchOMDb(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "tmdb":
		if s.apiKey == "" {
			return errors.New("tmdb provider not configured (set TMDB_API_KEY)")
		}
		info, err := s.searchTMDB(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	case "tvmaze":
		info, err := s.searchTVMaze(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	case "anilist":
		info, err := s.searchAniList(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	case "wikipedia":
		info, err := s.searchWikipedia(ctx, title, year)
		if err != nil {
			return err
		}
		if info.Title == "" {
			return fmt.Errorf("no metadata match found for %q", title)
		}
		return s.apply(ctx, videoID, info)
	}
	if !s.Enabled() {
		return errors.New("no metadata provider configured (set TMDB_API_KEY or keep TVMAZE_ENABLED/ANILIST_ENABLED=1)")
	}
	if err := s.rateLimit(ctx); err != nil {
		return err
	}
	info, err := s.search(ctx, title, year)
	if err != nil {
		return err
	}
	if info.Title == "" {
		return fmt.Errorf("no metadata match found for %q", title)
	}
	return s.apply(ctx, videoID, info)
}

// searchCustom queries the configured SCRAPE_CUSTOM_URL template (with %s
// replaced by the URL-escaped title) and parses a JSON metadata object.
func (s *Scraper) searchCustom(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	u := strings.ReplaceAll(s.customURL, "%s", url.QueryEscape(title))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("custom scraper: HTTP %d", resp.StatusCode)
	}
	var d struct {
		Title       string   `json:"title"`
		Year        int      `json:"year"`
		Synopsis    string   `json:"synopsis"`
		Genres      []string `json:"genres"`
		PosterURL   string   `json:"poster_url"`
		BackdropURL string   `json:"backdrop_url"`
		TrailerURL  string   `json:"trailer_url"`
		TrailerName string   `json:"trailer_title"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&d); err != nil {
		return nil, fmt.Errorf("custom scraper: parse response: %w", err)
	}
	info := &tmdbInfo{
		Title: d.Title, Year: d.Year, Synopsis: d.Synopsis, Genres: d.Genres,
		Poster: d.PosterURL, Backdrop: d.BackdropURL,
		TrailerURL: d.TrailerURL, TrailerTitle: d.TrailerName,
	}
	if info.Year == 0 {
		info.Year = year
	}
	return info, nil
}

// searchOMDb queries the OMDb API (needs OMDB_API_KEY).
func (s *Scraper) searchOMDb(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	u := fmt.Sprintf("https://www.omdbapi.com/?apikey=%s&t=%s&plot=full",
		url.QueryEscape(s.omdbKey), url.QueryEscape(title))
	if year > 0 {
		u += "&y=" + strconv.Itoa(year)
	}
	var d struct {
		Title  string `json:"Title"`
		Year   string `json:"Year"`
		Plot   string `json:"Plot"`
		Genre  string `json:"Genre"`
		Poster string `json:"Poster"`
	}
	if err := s.getJSON(ctx, u, &d); err != nil {
		return nil, err
	}
	if d.Title == "" {
		return &tmdbInfo{}, nil
	}
	info := &tmdbInfo{Title: d.Title, Synopsis: d.Plot, Poster: d.Poster}
	info.Year = yearFromText(d.Year)
	for _, g := range strings.Split(d.Genre, ",") {
		if g = strings.TrimSpace(g); g != "" {
			info.Genres = append(info.Genres, g)
		}
	}
	return info, nil
}

type tmdbInfo struct {
	TmdbID       int
	Title        string
	Year         int
	Synopsis     string
	Genres       []string
	Poster       string
	Backdrop     string
	TrailerURL   string
	TrailerTitle string
}

func (s *Scraper) search(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	if s.apiKey != "" {
		return s.searchTMDB(ctx, title, year)
	}
	if tvmazeEnabled() {
		info, err := s.searchTVMaze(ctx, title, year)
		if err != nil {
			return nil, err
		}
		if info.TmdbID != 0 {
			return info, nil
		}
	}
	if anilistEnabled() {
		info, err := s.searchAniList(ctx, title, year)
		if err != nil {
			return nil, err
		}
		if info.TmdbID != 0 {
			return info, nil
		}
	}
	if wikipediaEnabled() {
		return s.searchWikipedia(ctx, title, year)
	}
	return &tmdbInfo{}, nil
}

func (s *Scraper) searchTMDB(ctx context.Context, title string, year int) (*tmdbInfo, error) {
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

	// backdrops for the detail-page hero banner
	var imagesResp struct {
		Backdrops []struct {
			FilePath string `json:"file_path"`
		} `json:"backdrops"`
	}
	iu := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/images?api_key=%s",
		best.ID, s.apiKey)
	if err := s.getJSON(ctx, iu, &imagesResp); err == nil && len(imagesResp.Backdrops) > 0 {
		info.Backdrop = imagesResp.Backdrops[0].FilePath
	}

	// Fanart.tv artwork enrichment (optional FANART_API_KEY)
	if s.fanartKey != "" {
		var fanartResp struct {
			Backdrops []struct {
				URL string `json:"url"`
			} `json:"moviebackground"`
			Posters []struct {
				URL string `json:"url"`
			} `json:"movieposter"`
		}
		fu := fmt.Sprintf("https://webservice.fanart.tv/v3/movies/%d?api_key=%s",
			best.ID, url.QueryEscape(s.fanartKey))
		if err := s.getJSON(ctx, fu, &fanartResp); err == nil {
			if info.Backdrop == "" && len(fanartResp.Backdrops) > 0 {
				info.Backdrop = fanartResp.Backdrops[0].URL
			}
			if info.Poster == "" && len(fanartResp.Posters) > 0 {
				info.Poster = fanartResp.Posters[0].URL
			}
		}
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

	// Official trailers: prefer the configured language, then any English
	// trailer, and keep the first YouTube result.
	var videosResp struct {
		Results []struct {
			Key      string `json:"key"`
			Name     string `json:"name"`
			Site     string `json:"site"`
			Type     string `json:"type"`
			Language string `json:"iso_639_1"`
		} `json:"results"`
	}
	vu := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/videos?api_key=%s&language=%s",
		best.ID, s.apiKey, s.lang)
	if err := s.getJSON(ctx, vu, &videosResp); err == nil {
		for _, pass := range []string{s.lang, "en", ""} {
			found := false
			for _, v := range videosResp.Results {
				if v.Site != "YouTube" || v.Type != "Trailer" {
					continue
				}
				if pass != "" && v.Language != pass {
					continue
				}
				info.TrailerURL = "https://www.youtube.com/watch?v=" + v.Key
				info.TrailerTitle = v.Name
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	return info, nil
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTagRe.ReplaceAllString(s, "")))
}

// searchTVMaze is the keyless fallback provider (free TV API, no key needed).
func (s *Scraper) searchTVMaze(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	u := "https://api.tvmaze.com/search/shows?q=" + urlQueryEscape(title)
	var results []struct {
		Show struct {
			ID        int      `json:"id"`
			Name      string   `json:"name"`
			Premiered string   `json:"premiered"`
			Genres    []string `json:"genres"`
			Summary   string   `json:"summary"`
			Image     struct {
				Medium string `json:"medium"`
			} `json:"image"`
		} `json:"show"`
	}
	if err := s.getJSON(ctx, u, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return &tmdbInfo{}, nil
	}
	show := results[0].Show
	if year > 0 {
		for _, r := range results {
			if yearFromDate(r.Show.Premiered) == year {
				show = r.Show
				break
			}
		}
	}
	return &tmdbInfo{
		TmdbID:   show.ID,
		Title:    show.Name,
		Year:     yearFromDate(show.Premiered),
		Synopsis: stripHTML(show.Summary),
		Genres:   show.Genres,
		Poster:   show.Image.Medium,
	}, nil
}

// searchAniList is the third keyless metadata provider (anime/animation via the
// AniList GraphQL API).
func (s *Scraper) searchAniList(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	query := `query ($q: String) {
		Media(search: $q, type: ANIME) {
			id
			title { romaji english native }
			startDate { year }
			genres
			description(asHtml: false)
			coverImage { medium }
		}
	}`
	payload, _ := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]any{"q": title},
	})
	var out struct {
		Data struct {
			Media struct {
				ID        int                                      `json:"id"`
				Title     struct{ Romaji, English, Native string } `json:"title"`
				StartDate struct{ Year int }                       `json:"startDate"`
				Genres    []string                                 `json:"genres"`
				Desc      string                                   `json:"description"`
				Cover     struct{ Medium string }                  `json:"coverImage"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := s.postJSON(ctx, "https://graphql.anilist.co", payload, &out); err != nil {
		return nil, err
	}
	m := out.Data.Media
	if m.ID == 0 {
		return &tmdbInfo{}, nil
	}
	name := m.Title.English
	if name == "" {
		name = m.Title.Romaji
	}
	if name == "" {
		name = m.Title.Native
	}
	return &tmdbInfo{
		TmdbID:   m.ID,
		Title:    name,
		Year:     m.StartDate.Year,
		Synopsis: stripHTML(m.Desc),
		Genres:   m.Genres,
		Poster:   m.Cover.Medium,
	}, nil
}

// searchWikipedia is the generic keyless last-resort provider: it finds
// candidate pages via the MediaWiki search API and picks the first summary
// matching the release year (when known).
func (s *Scraper) searchWikipedia(ctx context.Context, title string, year int) (*tmdbInfo, error) {
	lang := wikipediaLang()
	su := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=query&format=json&list=search&srsearch=%s&srlimit=5",
		lang, urlQueryEscape(title))
	var sr struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := s.getJSON(ctx, su, &sr); err != nil {
		return nil, err
	}
	if len(sr.Query.Search) == 0 {
		return &tmdbInfo{}, nil
	}
	var best *tmdbInfo
	for _, cand := range sr.Query.Search {
		info, err := s.wikipediaSummary(ctx, lang, cand.Title)
		if err != nil {
			continue
		}
		if best == nil {
			best = info
		}
		if year == 0 || info.Year == year {
			return info, nil
		}
	}
	if best != nil {
		return best, nil
	}
	return &tmdbInfo{}, nil
}

func (s *Scraper) wikipediaSummary(ctx context.Context, lang, pageTitle string) (*tmdbInfo, error) {
	u := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s",
		lang, url.PathEscape(pageTitle))
	var out struct {
		Title   string `json:"title"`
		Extract string `json:"extract"`
		Thumb   struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
		Description string `json:"description"`
	}
	if err := s.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	if out.Title == "" && out.Extract == "" {
		return &tmdbInfo{}, nil
	}
	info := &tmdbInfo{
		Title:    out.Title,
		Synopsis: out.Extract,
		Poster:   out.Thumb.Source,
		Year:     yearFromText(out.Description + " " + out.Extract),
	}
	return info, nil
}

var yearTextRe = regexp.MustCompile(`\b(1[89]\d{2}|20\d{2})\b`)

func yearFromText(s string) int {
	if m := yearTextRe.FindString(s); m != "" {
		if y, err := strconv.Atoi(m); err == nil {
			return y
		}
	}
	return 0
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
	backdropPath := ""
	if info.Backdrop != "" {
		if p, err := s.downloadImage(ctx, videoID, info.Backdrop, "backdrops", "w1280"); err == nil {
			backdropPath = p
		} else {
			log.Printf("[scrape:%s] backdrop download failed: %v", videoID.String()[:8], err)
		}
	}
	var oldPoster, oldBackdrop string
	err := s.pool.QueryRow(ctx,
		`SELECT poster_path, backdrop_path FROM videos WHERE id=$1`, videoID).
		Scan(&oldPoster, &oldBackdrop)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE videos SET title=$1, year=$2, synopsis=$3, genres=$4, poster_path=$5,
			backdrop_path=$6, tmdb_id=$7, trailer_url=$8, trailer_title=$9,
			scraped_at=now(), updated_at=now() WHERE id=$10`,
		info.Title, info.Year, info.Synopsis, info.Genres, posterPath, backdropPath,
		info.TmdbID, info.TrailerURL, info.TrailerTitle, videoID); err != nil {
		return err
	}
	if oldPoster != "" && oldPoster != posterPath &&
		filepath.Dir(oldPoster) == filepath.Join(s.dataDir, "posters") {
		_ = os.Remove(oldPoster)
	}
	if oldBackdrop != "" && oldBackdrop != backdropPath &&
		filepath.Dir(oldBackdrop) == filepath.Join(s.dataDir, "backdrops") {
		_ = os.Remove(oldBackdrop)
	}
	return nil
}

func (s *Scraper) downloadPoster(ctx context.Context, videoID uuid.UUID, path string) (string, error) {
	return s.downloadImage(ctx, videoID, path, "posters", "w500")
}

// downloadImage fetches a TMDB image path into DATA_DIR/<kind>/<videoID>.<ext>.
func (s *Scraper) downloadImage(ctx context.Context, videoID uuid.UUID, path, kind, size string) (string, error) {
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = "https://image.tmdb.org/t/p/" + size + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
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
	dir := filepath.Join(s.dataDir, kind)
	_ = os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, videoID.String()+ext)
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tmdb http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (s *Scraper) postJSON(ctx context.Context, url string, payload []byte, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("provider http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
			fmt.Fprintf(&b, "%%%02X", r)
		}
	}
	return b.String()
}
