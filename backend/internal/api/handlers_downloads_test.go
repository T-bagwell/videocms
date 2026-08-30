package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDownloadJobLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()

	status, d := doJSON(t, "POST", env.server.URL+"/api/downloads", token,
		map[string]any{"url": "https://example.com/video", "target_path": target})
	if status != http.StatusCreated {
		t.Fatalf("create download status = %d, body = %v", status, d)
	}
	id, _ := d["id"].(string)
	if id == "" {
		t.Fatal("create download missing id")
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/downloads", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list downloads status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("downloads count = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if first["status"] != "queued" {
		t.Fatalf("job status = %v, want queued", first["status"])
	}

	// A queued job cannot be retried yet.
	status, _ = doJSON(t, "POST", env.server.URL+"/api/downloads/"+id+"/retry", token, nil)
	if status != http.StatusConflict {
		t.Fatalf("retry queued status = %d, want 409", status)
	}

	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/downloads/"+id, token, nil)
	if status != http.StatusOK {
		t.Fatalf("cancel status = %d", status)
	}
	status, _ = doJSON(t, "POST", env.server.URL+"/api/downloads/"+id+"/retry", token, nil)
	if status != http.StatusOK {
		t.Fatalf("retry canceled status = %d", status)
	}

	// Validation: bad scheme and relative target.
	status, _ = doJSON(t, "POST", env.server.URL+"/api/downloads", token,
		map[string]any{"url": "ftp://example.com/v", "target_path": target})
	if status != http.StatusBadRequest {
		t.Fatalf("bad scheme status = %d, want 400", status)
	}
	status, _ = doJSON(t, "POST", env.server.URL+"/api/downloads", token,
		map[string]any{"url": "https://example.com/v", "target_path": "relative"})
	if status != http.StatusBadRequest {
		t.Fatalf("relative target status = %d, want 400", status)
	}
}

func writeFakeBin(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-yt-dlp")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func waitDownloadStatus(t *testing.T, env *integrationEnv, id uuid.UUID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		err := env.pool.QueryRow(context.Background(),
			`SELECT status FROM downloads WHERE id=$1`, id).Scan(&status)
		if err == nil && status == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	var status string
	_ = env.pool.QueryRow(context.Background(),
		`SELECT status FROM downloads WHERE id=$1`, id).Scan(&status)
	t.Fatalf("download status = %q, want %q", status, want)
}

func TestDownloaderRunsJob(t *testing.T) {
	env := newIntegrationEnv(t)
	target := t.TempDir()
	env.app.dl.SetBin(writeFakeBin(t, "exit 0"))

	var id uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		INSERT INTO downloads (url, target_path, format)
		VALUES ('https://example.com/v', $1, 'best') RETURNING id`, target).Scan(&id); err != nil {
		t.Fatalf("insert download: %v", err)
	}
	env.app.dl.ProcessNext(context.Background())
	waitDownloadStatus(t, env, id, "completed")
}

func TestDownloaderFailsJob(t *testing.T) {
	env := newIntegrationEnv(t)
	target := t.TempDir()
	env.app.dl.SetBin(writeFakeBin(t, "echo boom 1>&2; exit 1"))

	var id uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `
		INSERT INTO downloads (url, target_path, format)
		VALUES ('https://example.com/v', $1, 'best') RETURNING id`, target).Scan(&id); err != nil {
		t.Fatalf("insert download: %v", err)
	}
	env.app.dl.ProcessNext(context.Background())
	waitDownloadStatus(t, env, id, "failed")

	var errMsg string
	_ = env.pool.QueryRow(context.Background(),
		`SELECT error FROM downloads WHERE id=$1`, id).Scan(&errMsg)
	if !strings.Contains(errMsg, "boom") {
		t.Fatalf("error message = %q, want to contain boom", errMsg)
	}
}

func TestDownloaderCancel(t *testing.T) {
	env := newIntegrationEnv(t)
	target := t.TempDir()
	env.app.dl.SetBin(writeFakeBin(t, "sleep 30"))
	token := loginAdmin(t, env)

	status, d := doJSON(t, "POST", env.server.URL+"/api/downloads", token,
		map[string]any{"url": "https://example.com/slow", "target_path": target})
	_ = status
	id, _ := d["id"].(string)
	jobID, _ := uuid.Parse(id)

	go env.app.dl.ProcessNext(context.Background())
	waitDownloadStatus(t, env, jobID, "downloading")

	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/downloads/"+id, token, nil)
	if status != http.StatusOK {
		t.Fatalf("cancel status = %d", status)
	}
	waitDownloadStatus(t, env, jobID, "canceled")
}
