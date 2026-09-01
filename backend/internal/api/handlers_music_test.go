package api

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
	"videocms/backend/internal/models"
)

// makeTestAudio writes a tiny tagged mp3; cover=true embeds cover art.
func makeTestAudio(t *testing.T, name, artist, album, title string, cover bool) (string, bool) {
	t.Helper()
	ffmpeg := media.ResolveTool("ffmpeg")
	dir := t.TempDir()
	src := filepath.Join(dir, name)
	args := []string{"-v", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:a", "libmp3lame", "-b:a", "64k",
		"-metadata", "artist=" + artist,
		"-metadata", "album=" + album,
		"-metadata", "title=" + title}
	if cover {
		coverPath := filepath.Join(dir, "cover.jpg")
		if out, err := exec.Command(ffmpeg, "-v", "error", "-y",
			"-f", "lavfi", "-i", "color=c=blue:size=64x64", "-frames:v", "1", coverPath).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg cover: %v: %s", err, out)
		}
		plain := filepath.Join(dir, "plain.mp3")
		args = append(args, plain)
		if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
			t.Skipf("ffmpeg cannot create mp3 (skipping): %v: %s", err, out)
		}
		final := filepath.Join(dir, "final.mp3")
		if _, err := exec.Command(ffmpeg, "-v", "error", "-y",
			"-i", plain, "-i", coverPath,
			"-map", "0:a", "-map", "1:v", "-c:a", "copy", "-c:v", "mjpeg",
			"-id3v2_version", "3", "-metadata:s:v", "title=Album cover", final).CombinedOutput(); err != nil {
			// cover embedding unsupported by this build: keep the tagged mp3
			os.Remove(final)
			if err := os.Rename(plain, src); err != nil {
				t.Fatal(err)
			}
			return src, false
		}
		os.Remove(plain)
		if err := os.Rename(final, src); err != nil {
			t.Fatal(err)
		}
		return src, true
	}
	args = append(args, src)
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot create mp3 (skipping): %v: %s", err, out)
	}
	return src, false
}

func TestMusicAlbumsFromScan(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()
	libDir := t.TempDir()

	tracks := []struct {
		name, artist, album, title string
		cover                      bool
	}{
		{"Night City.mp3", "Synthwave", "Retro Nights", "Night City", true},
		{"Neon Drive.mp3", "Synthwave", "Retro Nights", "Neon Drive", false},
		{"Morning.mp3", "Lofi", "Morning Beats", "Morning", false},
	}
	coverEmbedded := false
	for _, tr := range tracks {
		src, ok := makeTestAudio(t, tr.name, tr.artist, tr.album, tr.title, tr.cover)
		if tr.cover && ok {
			coverEmbedded = true
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(libDir, tr.name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var libID uuid.UUID
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, path) VALUES ($1, $2) RETURNING id`,
		"music", libDir).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	if !env.app.scanner.Start(ctx, models.Library{ID: libID, Name: "music", Path: libDir}) {
		t.Fatal("scan did not start")
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		var finished *time.Time
		if err := env.pool.QueryRow(ctx,
			`SELECT scan_finished_at FROM libraries WHERE id=$1`, libID).Scan(&finished); err != nil {
			t.Fatal(err)
		}
		if finished != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var status string
	var finished *time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT scan_status, scan_finished_at FROM libraries WHERE id=$1`, libID).
		Scan(&status, &finished); err != nil {
		t.Fatal(err)
	}
	if finished == nil {
		t.Fatal("scan did not finish in time")
	}

	token := loginAdmin(t, env)
	var albums struct {
		Items []models.Album `json:"items"`
	}
	resp := env.doJSON(t, http.MethodGet, "/api/albums", nil, token, &albums)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("albums status = %d", resp.StatusCode)
	}
	if len(albums.Items) != 2 {
		t.Fatalf("album count = %d, want 2 (%+v)", len(albums.Items), albums.Items)
	}
	var retro *models.Album
	for i := range albums.Items {
		if albums.Items[i].Name == "Retro Nights" {
			retro = &albums.Items[i]
		}
	}
	if retro == nil {
		t.Fatalf("Retro Nights album missing: %+v", albums.Items)
	}
	if retro.Artist != "Synthwave" || retro.TrackCount != 2 {
		t.Errorf("retro album = %+v, want Synthwave with 2 tracks", retro)
	}
	if coverEmbedded && !retro.HasCover {
		t.Error("Retro Nights should have an extracted cover")
	}

	var detail struct {
		Album  models.Album        `json:"album"`
		Tracks []models.MusicTrack `json:"tracks"`
	}
	resp = env.doJSON(t, http.MethodGet, "/api/albums/"+retro.ID.String(), nil, token, &detail)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("album detail status = %d", resp.StatusCode)
	}
	if len(detail.Tracks) != 2 {
		t.Fatalf("album track count = %d, want 2", len(detail.Tracks))
	}
	if detail.Tracks[0].Album != "Retro Nights" || detail.Tracks[0].Artist != "Synthwave" {
		t.Errorf("track metadata = %+v", detail.Tracks[0])
	}

	if coverEmbedded {
		resp = env.doJSON(t, http.MethodGet, "/api/albums/"+retro.ID.String()+"/poster", nil, token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("album poster status = %d, want 200", resp.StatusCode)
		}
	}
}
