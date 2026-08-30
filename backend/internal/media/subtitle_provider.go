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
	"strings"
	"sync"
	"time"
)

// SubtitleCandidate is one downloadable subtitle file from a provider.
type SubtitleCandidate struct {
	ID       string `json:"id"`
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
