package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestVideoThumbnails(t *testing.T) {
	env := newIntegrationEnv(t)
	src := makeTestVideoDur(t, 12)
	libID := env.insertLibrary(t, false)
	videoID := env.insertRealVideo(t, libID, "thumbs", src)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET duration_sec=30 WHERE id=$1`, videoID); err != nil {
		t.Fatalf("set duration: %v", err)
	}
	token := loginAdmin(t, env)

	status, d := doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/thumbnails", token, nil)
	if status != http.StatusOK {
		t.Fatalf("thumbnails meta status = %d, body = %v", status, d)
	}
	if d["count"].(float64) < 1 {
		t.Fatalf("thumbnail count = %v, want >= 1", d["count"])
	}
	if d["interval_sec"].(float64) != 10 || d["width"].(float64) != 160 {
		t.Fatalf("meta = %v", d)
	}

	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/api/videos/%s/thumbnails/1", env.server.URL, videoID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("thumbnail frame request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("frame status = %d: %s", res.StatusCode, b)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("frame Content-Type = %q, want image/jpeg", ct)
	}

	status, _ = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/thumbnails/999", token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("out-of-range frame status = %d, want 404", status)
	}
	status, _ = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String()+"/thumbnails/0", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("index 0 status = %d, want 400", status)
	}
}
