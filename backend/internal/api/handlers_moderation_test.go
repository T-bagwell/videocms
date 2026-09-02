package api

import (
	"net/http"
	"testing"
)

func TestModerationReports(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Content", ".mp4")
	base := env.server.URL + "/api/videos/" + videoID.String() + "/report"

	status, d := doJSON(t, http.MethodPost, base, token,
		map[string]any{"reason": "spam", "details": "looks fake"})
	if status != http.StatusCreated {
		t.Fatalf("report status = %d body = %v", status, d)
	}
	status, _ = doJSON(t, http.MethodPost, base, token,
		map[string]any{"reason": "spam"})
	if status != http.StatusConflict {
		t.Fatalf("duplicate report status = %d, want 409", status)
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/reports", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("reports status = %d body = %v", status, d)
	}
	rep := d["items"].([]any)[0].(map[string]any)
	if rep["status"] != "pending" || rep["video_title"] != "Content" {
		t.Fatalf("report = %v", rep)
	}
	status, _ = doJSON(t, http.MethodPost,
		env.server.URL+"/api/admin/reports/"+rep["id"].(string)+"/decide", token,
		map[string]any{"status": "reviewed"})
	if status != http.StatusOK {
		t.Fatalf("decide status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/reports", token, nil)
	if d["items"].([]any)[0].(map[string]any)["status"] != "reviewed" {
		t.Fatalf("report status after decide = %v", d)
	}
}

func TestModerationMuteBlocksComments(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Content", ".mp4")
	commentsURL := env.server.URL + "/api/videos/" + videoID.String() + "/comments"

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/auth/register", "",
		map[string]any{"username": "user2", "password": "secret1"})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d body = %v", status, d)
	}
	user2Token := d["token"].(string)
	user2ID := d["user"].(map[string]any)["id"].(string)

	status, _ = doJSON(t, http.MethodPost, commentsURL, user2Token, map[string]any{"body": "hello"})
	if status != http.StatusCreated {
		t.Fatalf("comment status = %d", status)
	}
	status, _ = doJSON(t, http.MethodPatch,
		env.server.URL+"/api/admin/users/"+user2ID+"/moderation", token,
		map[string]any{"muted": true})
	if status != http.StatusOK {
		t.Fatalf("mute status = %d", status)
	}
	status, _ = doJSON(t, http.MethodPost, commentsURL, user2Token, map[string]any{"body": "nope"})
	if status != http.StatusForbidden {
		t.Fatalf("muted comment status = %d, want 403", status)
	}
	status, d = doJSON(t, http.MethodGet, commentsURL, token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 0 {
		t.Fatalf("muted user comments should be hidden: %v", d)
	}
}
