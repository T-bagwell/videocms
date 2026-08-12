package media

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalkVideos(t *testing.T) {
	root := t.TempDir()

	mustWrite := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite(filepath.Join(root, "movie.mp4"), "x")
	mustWrite(filepath.Join(root, "series", "Show S01E01.mkv"), "x")
	mustWrite(filepath.Join(root, "series", "Show S01E02.mkv"), "x")
	mustWrite(filepath.Join(root, "notes.txt"), "x")
	mustWrite(filepath.Join(root, "movie.srt"), "x") // subtitle sidecar, not a video
	mustWrite(filepath.Join(root, ".hidden.mkv"), "x")
	mustWrite(filepath.Join(root, ".Trashes", "trash.mp4"), "x")
	mustWrite(filepath.Join(root, "show.m3u8", "index", "0.ts"), "x")
	mustWrite(filepath.Join(root, "show.m3u8", "index", "1.ts"), "x")

	var got []string
	err := walkVideos(root, func(path string, d fs.DirEntry) error {
		got = append(got, filepath.ToSlash(path))
		return nil
	})
	if err != nil {
		t.Fatalf("walkVideos: %v", err)
	}
	sort.Strings(got)

	want := []string{
		filepath.ToSlash(filepath.Join(root, "movie.mp4")),
		filepath.ToSlash(filepath.Join(root, "series", "Show S01E01.mkv")),
		filepath.ToSlash(filepath.Join(root, "series", "Show S01E02.mkv")),
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("walkVideos found %d files, want %d:\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("file %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWalkVideosMissingRoot(t *testing.T) {
	err := walkVideos(filepath.Join(t.TempDir(), "does-not-exist"), func(path string, d fs.DirEntry) error {
		return nil
	})
	if err == nil {
		t.Fatal("walkVideos on a missing root should return an error")
	}
}

func TestWalkVideosCallbackError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := os.ErrPermission
	err := walkVideos(root, func(path string, d fs.DirEntry) error {
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("walkVideos should propagate the callback error, got %v", err)
	}
}
