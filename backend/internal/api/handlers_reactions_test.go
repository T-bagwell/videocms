package api

import (
	"net/http"
	"testing"
)

func TestVideoReactions(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Reactable", ".mp4")
	base := env.server.URL + "/api/videos/" + videoID.String()

	status, _ := doJSON(t, http.MethodPut, base+"/reaction", token, map[string]any{"value": 1})
	if status != http.StatusOK {
		t.Fatalf("like status = %d", status)
	}
	status, d := doJSON(t, http.MethodGet, base+"/reactions", token, nil)
	if status != http.StatusOK || d["likes"].(float64) != 1 || d["mine"].(float64) != 1 {
		t.Fatalf("reactions after like = %v", d)
	}
	status, _ = doJSON(t, http.MethodPut, base+"/reaction", token, map[string]any{"value": -1})
	if status != http.StatusOK {
		t.Fatalf("dislike status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet, base+"/reactions", token, nil)
	if d["likes"].(float64) != 0 || d["dislikes"].(float64) != 1 || d["mine"].(float64) != -1 {
		t.Fatalf("reactions after dislike = %v", d)
	}
	status, _ = doJSON(t, http.MethodPut, base+"/reaction", token, map[string]any{"value": 0})
	status, d = doJSON(t, http.MethodGet, base+"/reactions", token, nil)
	if status != http.StatusOK || d["likes"].(float64) != 0 || d["mine"].(float64) != 0 {
		t.Fatalf("reactions after clear = %v", d)
	}
}

func TestPerItemPolicyToggles(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Gated", ".mp4")
	base := env.server.URL + "/api/videos/" + videoID.String()

	patch := func(body map[string]any) {
		t.Helper()
		status, d := doJSON(t, http.MethodPatch, base, token, body)
		if status != http.StatusOK {
			t.Fatalf("patch status = %d body = %v", status, d)
		}
	}

	// Disable downloads.
	patch(map[string]any{"title": "Gated", "allow_downloads": false})
	status, d := doJSON(t, http.MethodGet, base, token, nil)
	if status != http.StatusOK || d["allow_downloads"] != false {
		t.Fatalf("video after toggle = %v", d)
	}
	status, _ = doJSON(t, http.MethodGet, base+"/download", token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("download status = %d, want 403", status)
	}

	// Disable comments.
	patch(map[string]any{"title": "Gated", "allow_comments": false})
	status, _ = doJSON(t, http.MethodPost, base+"/comments", token, map[string]any{"body": "hi"})
	if status != http.StatusForbidden {
		t.Fatalf("comment status = %d, want 403", status)
	}

	// Disable reports.
	patch(map[string]any{"title": "Gated", "allow_reports": false})
	status, _ = doJSON(t, http.MethodPost, base+"/report", token, map[string]any{"reason": "spam"})
	if status != http.StatusForbidden {
		t.Fatalf("report status = %d, want 403", status)
	}
}
