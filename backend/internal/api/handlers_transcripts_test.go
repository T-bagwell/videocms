package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeFakeWhisper(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-whisper")
	script := `#!/bin/sh
out=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-of" ]; then out="$a"; fi
  prev="$a"
done
printf 'WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello transcript content\n' > "${out}.vtt"
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranscribeVideo(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.cfg.WhisperBin = writeFakeWhisper(t)
	env.app.cfg.WhisperModel = "model.bin"
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Lecture", ".mkv")
	token := loginAdmin(t, env)

	status, d := doJSON(t, "POST", env.server.URL+"/api/videos/"+videoID.String()+"/transcribe", token, nil)
	if status != http.StatusOK {
		t.Fatalf("transcribe status = %d, body = %v", status, d)
	}
	if d["status"] != "done" {
		t.Fatalf("transcript status = %v", d["status"])
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/transcripts", token, nil)
	if status != http.StatusOK || d["status"] != "done" {
		t.Fatalf("get transcript status = %d, body = %v", status, d)
	}
	preview, _ := d["preview"].(string)
	if len(preview) == 0 || preview[:5] != "Hello" {
		t.Fatalf("transcript preview = %q", preview)
	}

	// The transcript should be registered as a subtitle track.
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/subtitle-tracks", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tracks status = %d", status)
	}
	items, _ := d["items"].([]any)
	found := false
	for _, it := range items {
		if it.(map[string]any)["title"] == "Transcript" {
			found = true
		}
	}
	if !found {
		t.Fatal("transcript should be registered as a subtitle track")
	}

	// Search should match transcript text.
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos?q=transcript+content", token, nil)
	if status != http.StatusOK {
		t.Fatalf("search status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) == 0 {
		t.Fatal("search by transcript text returned no videos")
	}
}

func TestTranscribeRequiresConfig(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "x", ".mkv")
	token := loginAdmin(t, env)
	status, _ := doJSON(t, "POST", env.server.URL+"/api/videos/"+videoID.String()+"/transcribe", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("unconfigured transcribe status = %d, want 400", status)
	}
}
