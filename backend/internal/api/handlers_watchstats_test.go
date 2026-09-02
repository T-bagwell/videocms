package api

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWatchStatsDashboard(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Watched Movie", ".mp4")

	status, _ := doJSON(t, http.MethodPut, env.server.URL+"/api/users/me/progress", token,
		map[string]any{"video_id": videoID.String(), "position_sec": 600, "duration_sec": 1000})
	if status != http.StatusOK {
		t.Fatalf("save progress status = %d", status)
	}

	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/users/me/stats", token, nil)
	if status != http.StatusOK {
		t.Fatalf("user stats status = %d", status)
	}
	if d["total_watch_sec"].(float64) != 600 || d["plays"].(float64) != 1 || d["movies_watched"].(float64) != 1 {
		t.Fatalf("user stats = %v", d)
	}
	if len(d["weekly"].([]any)) == 0 {
		t.Fatal("weekly series missing")
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/stats/watch", token, nil)
	if status != http.StatusOK {
		t.Fatalf("admin stats status = %d", status)
	}
	if d["active_users"].(float64) != 1 || d["total_plays"].(float64) != 1 {
		t.Fatalf("admin stats = %v", d)
	}

	// CSV export.
	req, _ := http.NewRequest(http.MethodGet,
		env.server.URL+"/api/users/me/stats/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Watched Movie")) {
		t.Fatalf("export status = %d body = %s", resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("export content type = %q", resp.Header.Get("Content-Type"))
	}
}
