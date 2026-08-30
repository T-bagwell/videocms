package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParentalControlsAndQuota(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	v1 := env.insertVideo(t, libID, "Restricted", ".mkv")
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET content_rating='R' WHERE id=$1`, v1); err != nil {
		t.Fatal(err)
	}
	var adminID uuid.UUID
	if err := env.pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE username='admin'`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}

	status, _ := doJSON(t, "PATCH", env.server.URL+"/api/admin/users/"+adminID.String(), token,
		map[string]any{"allowed_rating": "G"})
	if status != http.StatusOK {
		t.Fatalf("set allowed rating status = %d", status)
	}

	status, d := doJSON(t, "GET", env.server.URL+"/api/videos", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list videos status = %d", status)
	}
	items, _ := d["items"].([]any)
	for _, it := range items {
		if it.(map[string]any)["title"] == "Restricted" {
			t.Fatal("restricted video should be filtered without unlock")
		}
	}

	status, _ = doJSON(t, "PUT", env.server.URL+"/api/users/me/pin", token, map[string]any{"pin": "1234"})
	if status != http.StatusOK {
		t.Fatalf("set pin status = %d", status)
	}
	status, d = doJSON(t, "POST", env.server.URL+"/api/users/me/pin/verify", token, map[string]any{"pin": "1234"})
	if status != http.StatusOK {
		t.Fatalf("verify pin status = %d, body = %v", status, d)
	}
	unlock, _ := d["unlock_token"].(string)
	if unlock == "" {
		t.Fatal("verify pin returned no unlock token")
	}
	status, _ = doJSON(t, "POST", env.server.URL+"/api/users/me/pin/verify", token, map[string]any{"pin": "9999"})
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong pin status = %d, want 401", status)
	}

	req, _ := http.NewRequest("GET", env.server.URL+"/api/videos", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Videocms-Unlock", unlock)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body := make([]byte, 1<<20)
	n, _ := res.Body.Read(body)
	if !strings.Contains(string(body[:n]), "Restricted") {
		t.Fatal("unlocked listing should include the restricted video")
	}

	// Library quota: upload over the quota is rejected.
	dir := t.TempDir()
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE libraries SET path=$1 WHERE id=$2`, dir, libID); err != nil {
		t.Fatal(err)
	}
	status, _ = doJSON(t, "PATCH", env.server.URL+"/api/libraries/"+libID.String(), token,
		map[string]any{"quota_bytes": 1000})
	if status != http.StatusOK {
		t.Fatalf("set quota status = %d", status)
	}
	status, d = doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "big.mp4", "target_path": dir, "size": 2000})
	if status != http.StatusBadRequest || !strings.Contains(d["error"].(string), "quota") {
		t.Fatalf("quota upload status = %d, body = %v", status, d)
	}
}
