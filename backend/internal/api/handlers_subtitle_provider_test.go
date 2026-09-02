package api

import (
	"context"
	"net/http"
	"testing"

	"videocms/backend/internal/media"
)

type queryGatedProvider struct {
	wantQuery string
}

func (f queryGatedProvider) Name() string { return "fake" }

func (f queryGatedProvider) Search(ctx context.Context, query, language string) ([]media.SubtitleCandidate, error) {
	if query != f.wantQuery {
		return nil, nil
	}
	return []media.SubtitleCandidate{{ID: "x", Language: language, Title: query}}, nil
}

func (f queryGatedProvider) Download(ctx context.Context, fileID string) ([]byte, error) {
	return []byte("x"), nil
}

func TestSubtitleSearchFuzzyFallback(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Movie", ".mkv")
	status, _ := doJSON(t, http.MethodPatch,
		env.server.URL+"/api/videos/"+videoID.String(), token,
		map[string]any{"title": "Movie (1999)", "year": 1999, "genres": []string{}})
	if status != http.StatusOK {
		t.Fatalf("patch status = %d", status)
	}

	// The provider only answers for the fuzzy (year-less) query.
	env.app.subProvider = queryGatedProvider{wantQuery: "Movie"}
	status, d := doJSON(t, http.MethodPost,
		env.server.URL+"/api/videos/"+videoID.String()+"/subtitles/search", token,
		map[string]any{"language": "en"})
	if status != http.StatusOK {
		t.Fatalf("search status = %d body = %v", status, d)
	}
	items := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("fuzzy fallback items = %d, want 1", len(items))
	}
	if items[0].(map[string]any)["provider"] != "fake" {
		t.Fatalf("provider = %v", items[0])
	}
}

func TestFuzzyTitleQuery(t *testing.T) {
	for in, want := range map[string]string{
		"Movie (1999) 1080p":        "Movie",
		"Show Name 2020 4K Web-DL":  "Show Name",
		"Plain Title":               "Plain Title",
		"Another 2012 Movie BluRay": "Another Movie",
	} {
		if got := fuzzyTitleQuery(in); got != want {
			t.Errorf("fuzzyTitleQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
