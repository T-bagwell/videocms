package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/config"
)

func TestRecordingsScheduler(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	src := makeTestVideo(t)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/iptv/channels", token,
		map[string]any{"name": "News", "source_url": srv.URL})
	if status != http.StatusCreated {
		t.Fatalf("create channel status = %d body = %v", status, d)
	}
	channelID := d["id"].(string)

	now := time.Now()
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/recordings", token,
		map[string]any{
			"channel_id": channelID,
			"title":      "Morning News",
			"start_utc":  now.Add(-2 * time.Minute).Format(time.RFC3339),
			"end_utc":    now.Add(5 * time.Minute).Format(time.RFC3339),
		})
	if status != http.StatusCreated {
		t.Fatalf("create recording status = %d body = %v", status, d)
	}
	recID := d["id"].(string)

	env.app.recorder.Process(context.Background())

	var recStatus, recErr, filePath string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := env.pool.QueryRow(context.Background(),
			`SELECT status, error, file_path FROM recordings WHERE id=$1`, recID).
			Scan(&recStatus, &recErr, &filePath); err != nil {
			t.Fatal(err)
		}
		if recStatus == "done" || recStatus == "failed" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if recStatus != "done" {
		t.Fatalf("recording status = %s error = %q", recStatus, recErr)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("recording file missing: %v", err)
	}

	// Catch-up: the finished recording is listed and streamable.
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/iptv/channels/"+channelID+"/catchup", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("catchup status = %d body = %v", status, d)
	}
	req, _ := http.NewRequest(http.MethodGet,
		env.server.URL+"/api/iptv/recordings/"+recID+"/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("recording stream status = %d len = %d", resp.StatusCode, len(body))
	}

	// Channel logo upload and serving.
	pngData := makeValidPNG(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("logo", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pngData); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost,
		env.server.URL+"/api/iptv/channels/"+channelID+"/logo", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logo upload status = %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/iptv/channels/"+channelID+"/logo", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	logoBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Equal(logoBody, pngData) {
		t.Fatalf("logo serve status = %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodGet, env.server.URL+"/api/iptv/channels.m3u?token="+token, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	m3uBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(m3uBody, []byte("/api/iptv/channels/"+channelID+"/logo")) {
		t.Fatalf("m3u missing absolute logo URL:\n%s", m3uBody)
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/recordings", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("recordings list status = %d body = %v", status, d)
	}
}

func TestTunerScan(t *testing.T) {
	lineup := `[{"GuideNumber":"1.1","GuideName":"ABC","URL":"http://fake/auto/v1"},` +
		`{"GuideNumber":"2.1","GuideName":"NBC","URL":"http://fake/auto/v2"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, lineup)
	}))
	defer srv.Close()

	env := newIntegrationEnv(t, func(c *config.Config) { c.HDHomeRunURLs = []string{srv.URL} })
	token := loginAdmin(t, env)
	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/admin/tuners/scan", token, nil)
	if status != http.StatusOK || int(d["created"].(float64)) != 2 {
		t.Fatalf("tuner scan status = %d body = %v", status, d)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/iptv/channels", token, nil)
	if status != http.StatusOK {
		t.Fatalf("channels status = %d", status)
	}
	names := map[string]bool{}
	for _, it := range d["items"].([]any) {
		m := it.(map[string]any)
		names[m["name"].(string)] = true
		if m["group_title"] != "Tuner" {
			t.Errorf("channel %v group = %v, want Tuner", m["name"], m["group_title"])
		}
	}
	if !names["ABC"] || !names["NBC"] {
		t.Fatalf("tuner channels missing: %v", names)
	}

	// List configured tuners.
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/tuners", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("tuners list status = %d body = %v", status, d)
	}
	_ = uuid.Nil
}
