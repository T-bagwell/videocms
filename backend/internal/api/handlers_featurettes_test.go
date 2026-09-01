package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestFeaturetteLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Movie", ".mkv")
	token := loginAdmin(t, env)

	// Admin uploads a featurette file with a custom title.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("title", "Behind the scenes"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "behind.mp4")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("fake featurette bytes")
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost,
		env.server.URL+"/api/videos/"+videoID.String()+"/featurettes", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", resp.StatusCode, data)
	}
	var created struct {
		ID        uuid.UUID `json:"id"`
		Title     string    `json:"title"`
		SizeBytes int64     `json:"size_bytes"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "Behind the scenes" {
		t.Errorf("title = %q, want %q", created.Title, "Behind the scenes")
	}
	if created.SizeBytes != int64(len(content)) {
		t.Errorf("size_bytes = %d, want %d", created.SizeBytes, len(content))
	}

	var filePath string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT file_path FROM featurettes WHERE id=$1`, created.ID).Scan(&filePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("featurette file missing on disk: %v", err)
	}

	// Listing shows the featurette.
	var list struct {
		Items []map[string]any `json:"items"`
	}
	resp = env.doJSON(t, http.MethodGet,
		"/api/videos/"+videoID.String()+"/featurettes", nil, token, &list)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	if len(list.Items) != 1 {
		t.Fatalf("featurette count = %d, want 1", len(list.Items))
	}

	// Streaming returns the exact uploaded bytes.
	req, err = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/featurettes/"+created.ID.String()+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("stream body = %q, want %q", body, content)
	}

	// Deleting removes both the record and the file.
	status, _ := doJSON(t, http.MethodDelete,
		env.server.URL+"/api/videos/"+videoID.String()+"/featurettes/"+created.ID.String(), token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("featurette file still exists after delete: %v", err)
	}
	var n int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM featurettes WHERE video_id=$1`, videoID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("featurette rows after delete = %d, want 0", n)
	}
}
