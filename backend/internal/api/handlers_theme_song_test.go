package api

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestThemeSongLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Movie", ".mkv")
	token := loginAdmin(t, env)

	upload := func(name, title, content string) int {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if title != "" {
			if err := mw.WriteField("title", title); err != nil {
				t.Fatal(err)
			}
		}
		fw, err := mw.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := mw.Close(); err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost,
			env.server.URL+"/api/videos/"+videoID.String()+"/theme-song", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	if status := upload("intro.mp3", "Main theme", "first"); status != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", status)
	}

	status, d := doJSON(t, http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/theme-song", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get theme song status = %d, body = %v", status, d)
	}
	if d["title"] != "Main theme" {
		t.Errorf("title = %v, want Main theme", d["title"])
	}

	// Streaming returns the uploaded bytes.
	req, err := http.NewRequest(http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/theme-song/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "first" {
		t.Fatalf("stream status = %d body = %q", resp.StatusCode, body)
	}

	// Replacing keeps a single row and serves the new bytes.
	if status := upload("outro.flac", "", "second"); status != http.StatusCreated {
		t.Fatalf("replace status = %d, want 201", status)
	}
	req, _ = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/theme-song/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "second" {
		t.Fatalf("replaced stream body = %q, want second", body)
	}
	var n int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM theme_songs WHERE video_id=$1`, videoID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("theme song rows = %d, want 1", n)
	}

	// Deleting removes it.
	status, _ = doJSON(t, http.MethodDelete,
		env.server.URL+"/api/videos/"+videoID.String()+"/theme-song", token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete status = %d", status)
	}
	status, _ = doJSON(t, http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/theme-song", token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", status)
	}
}
