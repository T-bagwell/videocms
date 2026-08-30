package api

import (
	"net/http"
	"testing"
)

func TestCommentsRatingsFeed(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Social", ".mkv")
	base := env.server.URL + "/api/videos/" + videoID.String()

	status, d := doJSON(t, "POST", base+"/comments", token, map[string]any{"body": "nice video"})
	if status != http.StatusCreated {
		t.Fatalf("add comment status = %d, body = %v", status, d)
	}
	commentID := d["id"].(string)
	status, d = doJSON(t, "GET", base+"/comments", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list comments status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("comment count = %d, want 1", len(items))
	}

	status, _ = doJSON(t, "PUT", base+"/rating", token, map[string]any{"stars": 4})
	if status != http.StatusOK {
		t.Fatalf("rate status = %d", status)
	}
	status, d = doJSON(t, "GET", base+"/ratings", token, nil)
	if status != http.StatusOK {
		t.Fatalf("ratings status = %d", status)
	}
	if d["mine"].(float64) != 4 || d["count"].(float64) != 1 || d["average"].(float64) != 4 {
		t.Fatalf("ratings = %v", d)
	}

	status, _ = doJSON(t, "POST", env.server.URL+"/api/users/me/favorites", token,
		map[string]any{"video_id": videoID.String()})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("favorite status = %d", status)
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/feed", token, nil)
	if status != http.StatusOK {
		t.Fatalf("feed status = %d", status)
	}
	items, _ = d["items"].([]any)
	hasComment := false
	hasFavorite := false
	for _, it := range items {
		m := it.(map[string]any)
		if m["kind"] == "comment" && m["video_title"] == "Social" {
			hasComment = true
		}
		if m["kind"] == "favorite" && m["video_title"] == "Social" {
			hasFavorite = true
		}
	}
	if !hasComment || !hasFavorite {
		t.Fatalf("feed missing comment/favorite: %v", items)
	}

	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/comments/"+commentID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete comment status = %d", status)
	}
}
