package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func loginAdmin(t *testing.T, env *integrationEnv) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	res, err := http.Post(env.server.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("login status %d: %s", res.StatusCode, b)
	}
	var d struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if d.Token == "" {
		t.Fatal("login returned empty token")
	}
	return d.Token
}

func doJSON(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = res.Body.Close() }()
	var d map[string]any
	_ = json.NewDecoder(res.Body).Decode(&d)
	return res.StatusCode, d
}

func TestUploadChunkedSession(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()

	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "movie.mp4", "target_path": target, "size": 0})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d, body = %v", status, d)
	}
	id, _ := d["id"].(string)
	if id == "" {
		t.Fatalf("create upload missing id: %v", d)
	}
	if cs, _ := d["chunk_size"].(float64); int64(cs) != defaultChunkSize {
		t.Fatalf("chunk_size = %v, want %d", cs, defaultChunkSize)
	}

	chunkURL := fmt.Sprintf("%s/api/uploads/%s/chunk/", env.server.URL, id)
	for i, payload := range []string{"0123456789", "abcdefghij"} {
		req, _ := http.NewRequest("PUT", chunkURL+fmt.Sprint(i), bytes.NewReader([]byte(payload)))
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("put chunk %d: %v", i, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("put chunk %d status = %d", i, res.StatusCode)
		}
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/uploads/"+id, token, nil)
	if status != http.StatusOK {
		t.Fatalf("get upload status = %d", status)
	}
	received, _ := d["received"].([]any)
	if len(received) != 2 {
		t.Fatalf("received chunks = %v, want 2", received)
	}

	status, d = doJSON(t, "POST", env.server.URL+"/api/uploads/"+id+"/complete", token, nil)
	if status != http.StatusOK {
		t.Fatalf("complete status = %d, body = %v", status, d)
	}
	finalPath := filepath.Join(target, "movie.mp4")
	b, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("final file missing: %v", err)
	}
	if string(b) != "0123456789abcdefghij" {
		t.Fatalf("final content = %q", b)
	}
}

func TestUploadVerifiesDeclaredSize(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()

	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "a.mp4", "target_path": target, "size": 10})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d", status)
	}
	id := d["id"].(string)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/uploads/%s/chunk/0", env.server.URL, id), bytes.NewReader([]byte("0123456789")))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ := http.DefaultClient.Do(req)
	_ = res.Body.Close()

	status, _ = doJSON(t, "POST", env.server.URL+"/api/uploads/"+id+"/complete", token, nil)
	if status != http.StatusOK {
		t.Fatalf("complete status = %d, want 200", status)
	}
}

func TestUploadRejectsIncomplete(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()

	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "a.mp4", "target_path": target, "size": 100})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d", status)
	}
	id := d["id"].(string)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/uploads/%s/chunk/0", env.server.URL, id), bytes.NewReader([]byte("0123456789")))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ := http.DefaultClient.Do(req)
	_ = res.Body.Close()

	status, _ = doJSON(t, "POST", env.server.URL+"/api/uploads/"+id+"/complete", token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("incomplete complete status = %d, want 400", status)
	}
	if _, err := os.Stat(filepath.Join(target, "a.mp4")); err == nil {
		t.Fatal("partial file must not be created")
	}
}

func TestUploadRequiresAbsoluteTarget(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "a.mp4", "target_path": "relative/dir", "size": 1})
	if status != http.StatusBadRequest {
		t.Fatalf("relative target status = %d, body = %v", status, d)
	}
}

func TestUploadSanitizesFilename(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	target := t.TempDir()

	status, d := doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "../../evil.mp4", "target_path": target, "size": 3})
	if status != http.StatusCreated {
		t.Fatalf("create upload status = %d, body = %v", status, d)
	}
	if name, _ := d["filename"].(string); name != "evil.mp4" {
		t.Fatalf("sanitized filename = %q, want evil.mp4", name)
	}
	id := d["id"].(string)

	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/api/uploads/%s/chunk/0", env.server.URL, id), bytes.NewReader([]byte("abc")))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ := http.DefaultClient.Do(req)
	_ = res.Body.Close()

	status, _ = doJSON(t, "POST", env.server.URL+"/api/uploads/"+id+"/complete", token, nil)
	if status != http.StatusOK {
		t.Fatalf("complete status = %d", status)
	}
	if _, err := os.Stat(filepath.Join(target, "evil.mp4")); err != nil {
		t.Fatalf("expected sanitized file in target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "..", "evil.mp4")); err == nil {
		t.Fatal("file escaped the target directory")
	}
}
