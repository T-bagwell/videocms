package api

import (
	"context"
	"net/http"
	"testing"

	"videocms/backend/internal/media"
)

type fakeSubtitleProvider struct{}

func (fakeSubtitleProvider) Name() string { return "fake" }

func (fakeSubtitleProvider) Search(_ context.Context, _, _ string) ([]media.SubtitleCandidate, error) {
	return []media.SubtitleCandidate{{ID: "42", Language: "en", Title: "Movie (2020)"}}, nil
}

func (fakeSubtitleProvider) Download(_ context.Context, _ string) ([]byte, error) {
	return []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"), nil
}

func TestSubtitleSearchAndDownload(t *testing.T) {
	env := newIntegrationEnv(t)
	env.app.subProvider = fakeSubtitleProvider{}
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Movie", ".mkv")
	token := loginAdmin(t, env)
	base := env.server.URL + "/api/videos/" + videoID.String() + "/subtitles"

	status, d := doJSON(t, "POST", base+"/search", token, map[string]any{"language": "en"})
	if status != http.StatusOK {
		t.Fatalf("search status = %d, body = %v", status, d)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search items = %d, want 1", len(items))
	}

	status, d = doJSON(t, "POST", base+"/download", token, map[string]any{"file_id": "42"})
	if status != http.StatusCreated {
		t.Fatalf("download status = %d, body = %v", status, d)
	}
	if d["format"] != "srt" {
		t.Fatalf("downloaded format = %v, want srt", d["format"])
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/subtitle-tracks", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tracks status = %d", status)
	}
	items, _ = d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("track count = %d, want 1", len(items))
	}
	track := items[0].(map[string]any)
	if track["kind"] != "upload" || track["format"] != "srt" {
		t.Fatalf("track = %v", track)
	}
}
