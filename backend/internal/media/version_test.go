package media

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMovieVersion(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		title     string
		width     int
		height    int
		wantKey   string
		wantLabel string
		wantRank  int
	}{
		{
			name:      "1080p vs 4K same film",
			filename:  "The Matrix (1999) 1080p BluRay.mkv",
			title:     "The Matrix (1999)",
			width:     1920,
			height:    1080,
			wantKey:   "the matrix 1999",
			wantLabel: "1080p",
			wantRank:  30,
		},
		{
			name:      "4K extended cut ranks highest",
			filename:  "The Matrix (1999) 4K UHD Extended Cut.mkv",
			title:     "The Matrix (1999)",
			width:     3840,
			height:    2160,
			wantKey:   "the matrix 1999",
			wantLabel: "4K Extended Cut",
			wantRank:  51,
		},
		{
			name:      "director's cut label",
			filename:  "Blade Runner (1982) 1080p Directors Cut.mkv",
			title:     "Blade Runner (1982)",
			width:     1920,
			height:    1080,
			wantKey:   "blade runner 1982",
			wantLabel: "1080p Director's Cut",
			wantRank:  31,
		},
		{
			name:      "probe fallback label and rank",
			filename:  "Dune.mkv",
			title:     "Dune",
			width:     3840,
			height:    2160,
			wantKey:   "dune",
			wantLabel: "4K",
			wantRank:  50,
		},
		{
			name:      "theatrical vs extended share key",
			filename:  "Star Wars (1977) Theatrical.mkv",
			title:     "Star Wars (1977)",
			width:     0,
			height:    0,
			wantKey:   "star wars 1977",
			wantLabel: "Theatrical",
			wantRank:  1,
		},
		{
			name:      "web release marker stripped",
			filename:  "The Batman (2022) 2160p WEB-DL HDR10.mkv",
			title:     "The Batman (2022)",
			width:     3840,
			height:    2160,
			wantKey:   "the batman 2022",
			wantLabel: "4K",
			wantRank:  50,
		},
		{
			name:      "empty title has no key",
			filename:  "clip.mkv",
			title:     "",
			width:     0,
			height:    0,
			wantKey:   "",
			wantLabel: "",
			wantRank:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, label, rank := ParseMovieVersion(tt.filename, tt.title, tt.width, tt.height)
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
			if label != tt.wantLabel {
				t.Errorf("label = %q, want %q", label, tt.wantLabel)
			}
			if rank != tt.wantRank {
				t.Errorf("rank = %d, want %d", rank, tt.wantRank)
			}
		})
	}
}

func TestMovieVersionGrouping(t *testing.T) {
	// Files of the same film with different quality markers must collapse to
	// one key; unrelated movies must stay separate.
	pairs := map[string]string{
		"The Matrix (1999) 1080p.mkv":         "the matrix 1999",
		"The Matrix (1999) 4K Remux.mkv":      "the matrix 1999",
		"The Matrix Reloaded (2003).mkv":      "the matrix reloaded 2003",
		"Matrix (1999) Extended Cut.mkv":      "matrix 1999",
		"The Matrix (1999) Directors Cut.mkv": "the matrix 1999",
	}
	for filename, want := range pairs {
		title := strings.TrimSuffix(filename, filepath.Ext(filename))
		key, _, _ := ParseMovieVersion(filename, title, 0, 0)
		if key != want {
			t.Errorf("%s: key = %q, want %q", filename, key, want)
		}
	}
}
