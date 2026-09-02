package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TraktItem struct {
	TmdbID    int
	WatchedAt time.Time
}

// TraktClient pushes watch history to Trakt (server-level sync).
type TraktClient struct {
	ClientID     string
	AccessToken  string
	RefreshToken string
	HTTP         *http.Client
}

func NewTraktClient(clientID, accessToken, refreshToken string) *TraktClient {
	return &TraktClient{
		ClientID:     clientID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		HTTP:         &http.Client{Timeout: 20 * time.Second},
	}
}

// Refresh exchanges the refresh token for a new access token.
func (c *TraktClient) Refresh(ctx context.Context) error {
	if c.RefreshToken == "" {
		return fmt.Errorf("trakt refresh token not configured (TRAKT_REFRESH_TOKEN)")
	}
	body, _ := json.Marshal(map[string]string{
		"client_id":     c.ClientID,
		"refresh_token": c.RefreshToken,
		"grant_type":    "refresh_token",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.trakt.tv/oauth/token", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("trakt token refresh failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return err
	}
	if out.AccessToken != "" {
		c.AccessToken = out.AccessToken
	}
	if out.RefreshToken != "" {
		c.RefreshToken = out.RefreshToken
	}
	return nil
}

// SyncHistory pushes movies to Trakt, deduplicated by TMDB id (latest wins).
func (c *TraktClient) SyncHistory(ctx context.Context, items []TraktItem) (int, error) {
	if c.ClientID == "" || c.AccessToken == "" {
		return 0, fmt.Errorf("trakt not configured (set TRAKT_CLIENT_ID and TRAKT_ACCESS_TOKEN)")
	}
	payload, count := buildTraktPayload(items)
	if count == 0 {
		return 0, nil
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.trakt.tv/sync/history", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", c.ClientID)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("trakt sync failed: HTTP %d", resp.StatusCode)
	}
	return count, nil
}

func buildTraktPayload(items []TraktItem) (any, int) {
	byID := map[int]TraktItem{}
	for _, it := range items {
		if it.TmdbID <= 0 {
			continue
		}
		if cur, ok := byID[it.TmdbID]; !ok || it.WatchedAt.After(cur.WatchedAt) {
			byID[it.TmdbID] = it
		}
	}
	type movie struct {
		IDs       map[string]int `json:"ids"`
		WatchedAt string         `json:"watched_at"`
	}
	payload := struct {
		Movies []movie `json:"movies"`
	}{Movies: []movie{}}
	for id, it := range byID {
		payload.Movies = append(payload.Movies, movie{
			IDs:       map[string]int{"tmdb": id},
			WatchedAt: it.WatchedAt.UTC().Format(time.RFC3339),
		})
	}
	return payload, len(payload.Movies)
}
