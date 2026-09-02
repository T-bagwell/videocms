package media

import (
	"context"
	"testing"
)

type stubProvider struct {
	name      string
	candidate SubtitleCandidate
}

func (s stubProvider) Name() string { return s.name }

func (s stubProvider) Search(ctx context.Context, query, language string) ([]SubtitleCandidate, error) {
	return []SubtitleCandidate{{ID: "abc", Language: language, Title: query}}, nil
}

func (s stubProvider) Download(ctx context.Context, fileID string) ([]byte, error) {
	return []byte("sub:" + fileID), nil
}

func TestMultiProviderSearchAndDownload(t *testing.T) {
	m := &MultiProvider{Providers: []SubtitleProvider{
		stubProvider{name: "one"},
		stubProvider{name: "two"},
	}}
	items, err := m.Search(context.Background(), "Movie", "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].Provider != "one" || items[0].ID != "one:abc" {
		t.Fatalf("first item = %+v", items[0])
	}
	data, err := m.Download(context.Background(), "two:abc")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sub:abc" {
		t.Fatalf("download = %q", data)
	}
}

func TestPodnapisiName(t *testing.T) {
	if got := NewPodnapisiProvider().Name(); got != "podnapisi" {
		t.Fatalf("name = %q", got)
	}
}
