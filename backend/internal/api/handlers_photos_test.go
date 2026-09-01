package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
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

func makeTestJPEG(t *testing.T, path string) {
	t.Helper()
	ffmpeg := media.ResolveTool("ffmpeg")
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=1", "-frames:v", "1", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot create jpeg (skipping): %v: %s", err, out)
	}
}

// makeValidPNG builds a tiny but fully valid PNG so image.DecodeConfig (and
// browsers) can decode it.
func makeValidPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPhotoAlbumsFromScan(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()
	libDir := t.TempDir()
	vacation := filepath.Join(libDir, "Vacation")
	if err := os.MkdirAll(vacation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vacation, "photo1.png"), makeValidPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	makeTestJPEG(t, filepath.Join(vacation, "photo2.jpg"))

	var libID uuid.UUID
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, path) VALUES ($1, $2) RETURNING id`,
		"photos", libDir).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	if !env.app.scanner.Start(ctx, models.Library{ID: libID, Name: "photos", Path: libDir}) {
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
	var n int
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM photos WHERE library_id=$1 AND available`, libID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("photos scanned = %d, want 2", n)
	}

	token := loginAdmin(t, env)
	var albums struct {
		Items []models.PhotoAlbum `json:"items"`
	}
	resp := env.doJSON(t, http.MethodGet, "/api/photo-albums", nil, token, &albums)
	if resp.StatusCode != http.StatusOK || len(albums.Items) != 1 {
		t.Fatalf("photo albums status = %d items = %d", resp.StatusCode, len(albums.Items))
	}
	al := albums.Items[0]
	if al.Name != "Vacation" || al.PhotoCount != 2 || !al.HasCover {
		t.Fatalf("photo album = %+v", al)
	}

	var photos struct {
		Items []models.Photo `json:"items"`
	}
	resp = env.doJSON(t, http.MethodGet,
		"/api/photos?album_id="+al.ID.String(), nil, token, &photos)
	if resp.StatusCode != http.StatusOK || len(photos.Items) != 2 {
		t.Fatalf("photos status = %d items = %d", resp.StatusCode, len(photos.Items))
	}
	dimensions := 0
	for _, p := range photos.Items {
		if p.Width > 0 && p.Height > 0 {
			dimensions++
		}
		if p.AlbumID == nil || *p.AlbumID != al.ID {
			t.Errorf("photo album_id = %v, want %s", p.AlbumID, al.ID)
		}
	}
	if dimensions != 2 {
		t.Errorf("photos with dimensions = %d, want 2", dimensions)
	}

	resp = env.doJSON(t, http.MethodGet,
		"/api/photos/"+photos.Items[0].ID.String()+"/file", nil, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("photo file status = %d, want 200", resp.StatusCode)
	}
}
