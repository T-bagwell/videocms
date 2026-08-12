package media

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestSearchWikipedia exercises the keyless Wikipedia fallback provider. It is
// gated behind NETWORK_TEST=1 so normal test runs stay offline.
func TestSearchWikipedia(t *testing.T) {
	if os.Getenv("NETWORK_TEST") == "" {
		t.Skip("set NETWORK_TEST=1 to run network-dependent tests")
	}
	s := &Scraper{client: &http.Client{Timeout: 20 * time.Second}, lang: "en"}
	info, err := s.searchWikipedia(context.Background(), "Spirited Away", 2001)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title == "" {
		t.Fatalf("no match: %+v", info)
	}
	if info.Year != 2001 {
		t.Errorf("year = %d, want 2001", info.Year)
	}
	if info.Poster == "" || info.Synopsis == "" {
		t.Errorf("missing poster/synopsis: %+v", info)
	}
}

func TestYearFromText(t *testing.T) {
	if got := yearFromText("2001 film by Hayao Miyazaki"); got != 2001 {
		t.Errorf("yearFromText = %d, want 2001", got)
	}
	if got := yearFromText("no year here"); got != 0 {
		t.Errorf("yearFromText = %d, want 0", got)
	}
}
