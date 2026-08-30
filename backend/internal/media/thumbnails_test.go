package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThumbnailPathBounds(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"frame_001.jpg", "frame_002.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("jpeg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := CountThumbnails(dir); got != 2 {
		t.Fatalf("CountThumbnails = %d, want 2", got)
	}
	if p := ThumbnailPath(dir, 1); p == "" || filepath.Base(p) != "frame_001.jpg" {
		t.Fatalf("ThumbnailPath(1) = %q", p)
	}
	if p := ThumbnailPath(dir, 2); p == "" {
		t.Fatal("ThumbnailPath(2) should exist")
	}
	if p := ThumbnailPath(dir, 3); p != "" {
		t.Fatalf("ThumbnailPath(3) = %q, want empty", p)
	}
	if p := ThumbnailPath(dir, 0); p != "" {
		t.Fatalf("ThumbnailPath(0) = %q, want empty", p)
	}
}
