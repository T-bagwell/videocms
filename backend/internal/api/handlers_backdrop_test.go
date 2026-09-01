package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestBackdropUploadAndServe(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Movie", ".mkv")
	token := loginAdmin(t, env)

	status, _ := doJSON(t, http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/backdrop", token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("missing backdrop status = %d, want 404", status)
	}

	pngData := makeValidPNG(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("backdrop", "hero.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(pngData); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost,
		env.server.URL+"/api/videos/"+videoID.String()+"/backdrop", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload backdrop status = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/videos/"+videoID.String()+"/backdrop", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, pngData) {
		t.Fatalf("serve backdrop status = %d body len = %d", resp.StatusCode, len(body))
	}
}
