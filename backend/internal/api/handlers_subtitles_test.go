package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestSubtitleFormatHelper(t *testing.T) {
	cases := map[string]string{
		"/data/sub.en.ass": "ass",
		"/data/sub.zh.srt": "srt",
		"/data/x.vtt":      "vtt",
		"":                 "",
	}
	for in, want := range cases {
		if got := subtitleFormat(in); got != want {
			t.Errorf("subtitleFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSubtitleTrackFormatAndHLSExclusion(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "subs", ".mkv")
	ctx := context.Background()

	assID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO subtitle_tracks (id, video_id, position, lang, title, path, kind, source_key)
		VALUES ($1, $2, 0, '', 'ASS', '/tmp/x.ass', 'sidecar', 'ass-track')`,
		assID, videoID); err != nil {
		t.Fatalf("insert ass track: %v", err)
	}
	vttID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO subtitle_tracks (id, video_id, position, lang, title, path, kind, source_key)
		VALUES ($1, $2, 1, '', 'VTT', '/tmp/y.vtt', 'sidecar', 'vtt-track')`,
		vttID, videoID); err != nil {
		t.Fatalf("insert vtt track: %v", err)
	}

	token := loginAdmin(t, env)
	status, d := doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/subtitle-tracks", token, nil)
	if status != http.StatusOK {
		t.Fatalf("subtitle-tracks status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("track count = %d, want 2", len(items))
	}
	formats := map[string]string{}
	for _, it := range items {
		m := it.(map[string]any)
		formats[m["title"].(string)] = m["format"].(string)
	}
	if formats["ASS"] != "ass" || formats["VTT"] != "vtt" {
		t.Fatalf("formats = %v", formats)
	}

	v, ok := env.app.videoForHLS(ctx, videoID)
	if !ok {
		t.Fatal("videoForHLS returned not found")
	}
	for _, s := range v.Subs {
		if s.ID == assID.String() {
			t.Fatal("ASS track must not be advertised in the HLS subtitle group")
		}
	}
	foundVTT := false
	for _, s := range v.Subs {
		if s.ID == vttID.String() {
			foundVTT = true
		}
	}
	if !foundVTT {
		t.Fatal("VTT track should still be in the HLS subtitle group")
	}
}
