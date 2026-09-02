package media

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SubtitleCandidate is one downloadable subtitle file from a provider.
type SubtitleCandidate struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// SubtitleProvider searches online subtitle sources and downloads files.
type SubtitleProvider interface {
	Name() string
	Search(ctx context.Context, query, language string) ([]SubtitleCandidate, error)
	Download(ctx context.Context, fileID string) ([]byte, error)
}

// OpenSubtitlesProvider talks to the opensubtitles.com REST API. It requires an
// API key and (for search) a username/password login.
type OpenSubtitlesProvider struct {
	apiURL   string
	username string
	password string
	apiKey   string
	client   *http.Client

	mu    sync.Mutex
	token string
}

func NewOpenSubtitlesProvider(username, password, apiKey string) *OpenSubtitlesProvider {
	return &OpenSubtitlesProvider{
		apiURL:   "https://api.opensubtitles.com/api/v1",
		username: username,
		password: password,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenSubtitlesProvider) Name() string { return "opensubtitles" }

// MultiProvider searches every configured provider and dispatches downloads by
// the candidate's provider prefix.
type MultiProvider struct {
	Providers []SubtitleProvider
}

func (m *MultiProvider) Name() string { return "multi" }

func (m *MultiProvider) Search(ctx context.Context, query, language string) ([]SubtitleCandidate, error) {
	var out []SubtitleCandidate
	for _, p := range m.Providers {
		items, err := p.Search(ctx, query, language)
		if err != nil {
			continue
		}
		for _, it := range items {
			it.Provider = p.Name()
			it.ID = p.Name() + ":" + it.ID
			out = append(out, it)
		}
	}
	return out, nil
}

func (m *MultiProvider) Download(ctx context.Context, fileID string) ([]byte, error) {
	provider, id, ok := strings.Cut(fileID, ":")
	if !ok {
		return nil, fmt.Errorf("invalid file id")
	}
	for _, p := range m.Providers {
		if p.Name() == provider {
			return p.Download(ctx, id)
		}
	}
	return nil, fmt.Errorf("unknown subtitle provider %q", provider)
}

// PodnapisiProvider searches the keyless Podnapisi.net site (best effort).
type PodnapisiProvider struct {
	client *http.Client
	cache  sync.Map // candidate ID -> server-generated relative path
}

func NewPodnapisiProvider() *PodnapisiProvider {
	return &PodnapisiProvider{
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  sync.Map{},
	}
}

func (p *PodnapisiProvider) Name() string { return "podnapisi" }

func (p *PodnapisiProvider) Search(ctx context.Context, query, language string) ([]SubtitleCandidate, error) {
	u := "https://podnapisi.net/subtitles/search/advanced?keywords=" + url.QueryEscape(query)
	if language != "" {
		u += "&language=" + url.QueryEscape(language)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VideoCMS/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("podnapisi search: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	html := string(body)
	seen := map[string]bool{}
	var out []SubtitleCandidate
	for _, m := range subtitleLinkRe.FindAllStringSubmatch(html, -1) {
		href := m[1]
		if seen[href] {
			continue
		}
		seen[href] = true
		p.cache.Store(href, href)
		out = append(out, SubtitleCandidate{ID: href, Language: language, Title: query})
		if len(out) >= 20 {
			break
		}
	}
	return out, nil
}

func (p *PodnapisiProvider) Download(ctx context.Context, fileID string) ([]byte, error) {
	// The candidate ID is only a key into the server-populated search cache;
	// the fetched path is server-generated, so user input never reaches the
	// request URL.
	raw, ok := p.cache.Load(fileID)
	if !ok {
		return nil, fmt.Errorf("unknown subtitle candidate (run a search first)")
	}
	page, err := podnapisiPath(raw.(string))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VideoCMS/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	dl := subtitleDownloadRe.FindSubmatch(body)
	if dl == nil {
		return nil, fmt.Errorf("podnapisi download link not found")
	}
	dlURL, err := podnapisiPath(string(dl[1]))
	if err != nil {
		return nil, err
	}
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, err
	}
	dlReq.Header.Set("User-Agent", "VideoCMS/1.0")
	dlResp, err := p.client.Do(dlReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dlResp.Body.Close() }()
	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("podnapisi download: HTTP %d", dlResp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(dlResp.Body, 8<<20))
}

// podnapisiPath builds an absolute podnapisi.net URL from a relative path and
// rejects anything that could escape the fixed host (SSRF guard).
func podnapisiPath(path string) (string, error) {
	if path == "" || strings.Contains(path, "://") || strings.HasPrefix(path, "//") ||
		strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid podnapisi path")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "https://podnapisi.net" + path, nil
}

var (
	subtitleLinkRe     = regexp.MustCompile(`href="(/subtitles/[^"]+)"`)
	subtitleDownloadRe = regexp.MustCompile(`href="(/download/[^"]+)"`)
)

func (p *OpenSubtitlesProvider) postJSON(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", p.apiKey)
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("opensubtitles %s: HTTP %d: %s", path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}

func (p *OpenSubtitlesProvider) login(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" {
		return nil
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := p.postJSON(ctx, "/login", map[string]string{
		"username": p.username,
		"password": p.password,
	}, &out); err != nil {
		return err
	}
	p.token = out.Token
	return nil
}

func (p *OpenSubtitlesProvider) Search(ctx context.Context, query, language string) ([]SubtitleCandidate, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("subtitle provider not configured (set SUBTITLE_OS_API_KEY)")
	}
	if err := p.login(ctx); err != nil {
		return nil, err
	}
	params := map[string]any{"query": query}
	if language != "" {
		params["languages"] = language
	}
	var out struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Language    string `json:"language"`
				ReleaseName string `json:"release_name"`
				Files       []struct {
					FileID int `json:"file_id"`
				} `json:"files"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.postJSON(ctx, "/subtitles", params, &out); err != nil {
		return nil, err
	}
	items := []SubtitleCandidate{}
	for _, d := range out.Data {
		if len(d.Attributes.Files) == 0 {
			continue
		}
		items = append(items, SubtitleCandidate{
			ID:       fmt.Sprint(d.Attributes.Files[0].FileID),
			Language: d.Attributes.Language,
			Title:    d.Attributes.ReleaseName,
		})
	}
	return items, nil
}

func (p *OpenSubtitlesProvider) Download(ctx context.Context, fileID string) ([]byte, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("subtitle provider not configured (set SUBTITLE_OS_API_KEY)")
	}
	var out struct {
		Link string `json:"link"`
	}
	if err := p.postJSON(ctx, "/download", map[string]string{"file_id": fileID}, &out); err != nil {
		return nil, err
	}
	if out.Link == "" {
		return nil, fmt.Errorf("opensubtitles download returned no link")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, out.Link, nil)
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("opensubtitles file download: HTTP %d", res.StatusCode)
	}
	return decodeSubtitlePayload(raw)
}

// decodeSubtitlePayload unpacks a gzip or zip subtitle payload (as returned by
// opensubtitles.com) and returns the first subtitle file's content.
func decodeSubtitlePayload(raw []byte) ([]byte, error) {
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		return io.ReadAll(io.LimitReader(gz, 8<<20))
	}
	if len(raw) >= 4 && bytes.Equal(raw[:4], []byte("PK\x03\x04")) {
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			ext := strings.ToLower(f.Name[strings.LastIndexByte(f.Name, '.')+1:])
			if ext == "srt" || ext == "vtt" || ext == "ass" || ext == "ssa" || ext == "txt" {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(io.LimitReader(rc, 8<<20))
				_ = rc.Close()
				if err == nil {
					return data, nil
				}
			}
		}
		return nil, fmt.Errorf("no subtitle file found inside zip archive")
	}
	return raw, nil
}
