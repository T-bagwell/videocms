package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

// makeTestVideo generates a tiny H.264/AAC mp4 with ffmpeg, skipping the test
// when ffmpeg is not available.
func makeTestVideo(t *testing.T) string {
	t.Helper()
	ffmpeg := media.ResolveTool("ffmpeg")
	src := filepath.Join(t.TempDir(), "sample.mp4")
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-shortest", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot create test video (skipping): %v: %s", err, out)
	}
	return src
}

func (env *integrationEnv) insertRealVideo(t *testing.T, libID uuid.UUID, title, src string) uuid.UUID {
	t.Helper()
	st, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat video: %v", err)
	}
	videoID := uuid.New()
	if _, err := env.pool.Exec(context.Background(), `
		INSERT INTO videos (id, library_id, title, filename, file_path, size_bytes, available, container)
		VALUES ($1, $2, $3, $4, $5, $6, true, 'mp4')`,
		videoID, libID, title, filepath.Base(src), src, st.Size()); err != nil {
		t.Fatalf("insert video: %v", err)
	}
	return videoID
}

func TestVideoTracksAndRemux(t *testing.T) {
	env := newIntegrationEnv(t)
	src := makeTestVideo(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertRealVideo(t, libID, "sample", src)
	token := loginAdmin(t, env)

	status, d := doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/tracks", token, nil)
	if status != http.StatusOK {
		t.Fatalf("tracks status = %d, body = %v", status, d)
	}
	if c, _ := d["container"].(string); c != "mp4" {
		t.Fatalf("container = %q, want mp4", c)
	}
	audio, _ := d["audio"].([]any)
	if len(audio) == 0 {
		t.Fatal("expected at least one audio track in test video")
	}
	firstAudio := audio[0].(map[string]any)
	audioIdx := int(firstAudio["index"].(float64))

	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/api/videos/%s/download/remux?container=mkv&audio=%d", env.server.URL, videoID, audioIdx), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("remux request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("remux status = %d: %s", res.StatusCode, b)
	}
	if cd := res.Header.Get("Content-Disposition"); !bytes.Contains([]byte(cd), []byte(".mkv")) {
		t.Fatalf("Content-Disposition = %q, want .mkv", cd)
	}
	head := make([]byte, 4)
	n, _ := io.ReadFull(res.Body, head)
	if n == 4 && !bytes.Equal(head, []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		t.Fatalf("output is not a Matroska stream: % x", head)
	}
}

func TestRemuxRejectsBadParams(t *testing.T) {
	env := newIntegrationEnv(t)
	src := makeTestVideo(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertRealVideo(t, libID, "sample", src)
	token := loginAdmin(t, env)

	status, _ := doJSON(t, "GET",
		env.server.URL+"/api/videos/"+videoID.String()+"/download/remux?container=avi", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("bad container status = %d, want 400", status)
	}
	status, _ = doJSON(t, "GET",
		env.server.URL+"/api/videos/"+videoID.String()+"/download/remux?container=mkv&audio=999", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("bad audio status = %d, want 400", status)
	}
}
