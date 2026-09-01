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

// makeTestVideoWithChapters builds a short mp4 with two ffmpeg chapters.
func makeTestVideoWithChapters(t *testing.T) string {
	t.Helper()
	ffmpeg := media.ResolveTool("ffmpeg")
	dir := t.TempDir()
	src := filepath.Join(dir, "sample.mp4")
	meta := filepath.Join(dir, "chapters.txt")
	content := ";FFMETADATA1\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=2500\ntitle=Intro\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=2500\nEND=6000\ntitle=Main Feature\n"
	if err := os.WriteFile(meta, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=8:size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=8",
		"-i", meta, "-map_metadata", "2",
		"-c:v", "libx264", "-c:a", "aac", "-shortest", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot create chaptered video (skipping): %v: %s", err, out)
	}
	return src
}

func TestChaptersFromScan(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()
	libDir := t.TempDir()
	src := makeTestVideoWithChapters(t)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	name := "Chapter Movie.mp4"
	if err := os.WriteFile(filepath.Join(libDir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var libID uuid.UUID
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, path) VALUES ($1, $2) RETURNING id`,
		"chapters", libDir).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	if !env.app.scanner.Start(ctx, models.Library{ID: libID, Name: "chapters", Path: libDir}) {
		t.Fatal("scan did not start")
	}
	deadline := time.Now().Add(45 * time.Second)
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
	var videoID uuid.UUID
	err = env.pool.QueryRow(ctx,
		`SELECT id FROM videos WHERE library_id=$1 AND filename=$2`, libID, name).Scan(&videoID)
	if err != nil {
		t.Fatalf("scanned video not found: %v", err)
	}

	token := loginAdmin(t, env)
	var chapters struct {
		Items []struct {
			ID       uuid.UUID `json:"id"`
			Position int       `json:"position"`
			StartSec float64   `json:"start_sec"`
			EndSec   float64   `json:"end_sec"`
			Title    string    `json:"title"`
		} `json:"items"`
	}
	resp := env.doJSON(t, http.MethodGet,
		"/api/videos/"+videoID.String()+"/chapters", nil, token, &chapters)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chapters status = %d", resp.StatusCode)
	}
	if len(chapters.Items) != 2 {
		t.Fatalf("chapter count = %d, want 2 (%+v)", len(chapters.Items), chapters.Items)
	}
	if chapters.Items[0].Title != "Intro" || chapters.Items[0].StartSec != 0 {
		t.Errorf("first chapter = %+v, want Intro at 0", chapters.Items[0])
	}
	if chapters.Items[1].Title != "Main Feature" || chapters.Items[1].StartSec != 2.5 {
		t.Errorf("second chapter = %+v, want Main Feature at 2.5", chapters.Items[1])
	}
	if chapters.Items[1].EndSec <= chapters.Items[1].StartSec {
		t.Errorf("second chapter end = %v, want > start", chapters.Items[1].EndSec)
	}
}
