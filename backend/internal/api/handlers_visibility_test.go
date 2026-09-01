package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestPerItemVisibility(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Secret", ".mp4")
	token := loginAdmin(t, env)
	vbase := env.server.URL + "/api/videos/" + videoID.String()
	pubBase := env.server.URL + "/api/public/videos/" + videoID.String()

	patch := func(body map[string]any) int {
		t.Helper()
		status, d := doJSON(t, http.MethodPatch, vbase, token, body)
		if status != http.StatusOK {
			t.Fatalf("patch status = %d, body = %v", status, d)
		}
		return status
	}

	// Default private: anonymous access is rejected.
	status, _ := doJSON(t, http.MethodGet, pubBase, "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("private video anonymous status = %d, want 404", status)
	}

	// Public: listed anonymously and streamable.
	patch(map[string]any{"title": "Secret", "visibility": "public"})
	status, d := doJSON(t, http.MethodGet, pubBase, "", nil)
	if status != http.StatusOK || d["visibility"] != "public" {
		t.Fatalf("public video status = %d body = %v", status, d)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/public/videos", "", nil)
	if status != http.StatusOK {
		t.Fatalf("public list status = %d", status)
	}
	items, _ := d["items"].([]any)
	found := false
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && m["id"] == videoID.String() {
			found = true
		}
	}
	if !found {
		t.Fatal("public video missing from public listing")
	}
	req, _ := http.NewRequest(http.MethodGet, pubBase+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "video" {
		t.Fatalf("public stream status = %d body = %q", resp.StatusCode, body)
	}

	// Unlisted: hidden from authed listings, still reachable by link.
	patch(map[string]any{"title": "Secret", "visibility": "unlisted"})
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/videos?page_size=50", token, nil)
	if status != http.StatusOK {
		t.Fatalf("videos list status = %d", status)
	}
	for _, it := range d["items"].([]any) {
		if m, ok := it.(map[string]any); ok && m["id"] == videoID.String() {
			t.Fatal("unlisted video should be hidden from listings")
		}
	}
	status, _ = doJSON(t, http.MethodGet, pubBase, "", nil)
	if status != http.StatusOK {
		t.Fatalf("unlisted anonymous status = %d, want 200", status)
	}

	// Password protected: unlock flow.
	patch(map[string]any{"title": "Secret", "visibility": "password", "access_password": "secret"})
	status, _ = doJSON(t, http.MethodGet, pubBase, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("locked video anonymous status = %d, want 401", status)
	}
	status, _ = doJSON(t, http.MethodPost, pubBase+"/unlock", "", map[string]any{"password": "nope"})
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", status)
	}
	status, d = doJSON(t, http.MethodPost, pubBase+"/unlock", "", map[string]any{"password": "secret"})
	if status != http.StatusOK {
		t.Fatalf("unlock status = %d body = %v", status, d)
	}
	accessToken, _ := d["token"].(string)
	if accessToken == "" {
		t.Fatal("unlock returned no token")
	}
	req, _ = http.NewRequest(http.MethodGet, pubBase, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var pub map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&pub)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || pub["password_protected"] != false {
		t.Fatalf("unlocked video status = %d body = %v", resp.StatusCode, pub)
	}
	req, _ = http.NewRequest(http.MethodGet, pubBase+"/stream?vt="+accessToken, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "video" {
		t.Fatalf("locked stream with token status = %d body = %q", resp.StatusCode, body)
	}

	// Back to private.
	patch(map[string]any{"title": "Secret", "visibility": "private"})
	status, _ = doJSON(t, http.MethodGet, pubBase, "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("private again status = %d, want 404", status)
	}
}
