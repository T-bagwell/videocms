package media

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseProgress(t *testing.T) {
	cases := []struct {
		line string
		want float64
		ok   bool
	}{
		{"[download]  45.2% of 100.00MiB at 2.3MiB/s ETA 00:24", 45.2, true},
		{"[download] 100% of 1.00MiB", 100, true},
		{"[download] Destination: foo.mp4", 0, false},
		{"[info] something", 0, false},
	}
	for i, c := range cases {
		got, ok := parseProgress(c.line)
		if ok != c.ok || (ok && fmt.Sprintf("%.1f", got) != fmt.Sprintf("%.1f", c.want)) {
			t.Errorf("case %d: parseProgress(%q) = %v, %v; want %v, %v", i, c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestBuildYtDlpArgs(t *testing.T) {
	job := DownloadJob{
		ID:          uuid.New(),
		URL:         "https://example.com/channel",
		TargetPath:  "/tmp/dl",
		Format:      "best",
		Proxy:       "http://proxy:3128",
		CookiesPath: "/tmp/cookies.txt",
		Username:    "user",
		Password:    "pass",
		Kind:        "channel",
	}
	args := buildYtDlpArgs(job)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--proxy", "http://proxy:3128",
		"--cookies", "/tmp/cookies.txt",
		"--username", "user", "--password", "pass",
		"--yes-playlist", "--no-overwrites",
		"--download-archive",
		"https://example.com/channel",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %s", want, joined)
		}
	}
	if !strings.Contains(joined, filepath.Join("/tmp/dl", ".videocms-archive", job.ID.String()+".txt")) {
		t.Errorf("archive path missing: %s", joined)
	}

	plain := buildYtDlpArgs(DownloadJob{URL: "https://x", TargetPath: "/tmp", Format: "best", Kind: "video"})
	if strings.Contains(strings.Join(plain, " "), "--yes-playlist") {
		t.Errorf("plain video should not use playlist flags: %v", plain)
	}
}
