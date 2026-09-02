package api

import (
	"net/http"
	"testing"
)

func TestNotificationPrefs(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, http.MethodGet,
		env.server.URL+"/api/users/me/notification-prefs", token, nil)
	if status != http.StatusOK {
		t.Fatalf("get prefs status = %d", status)
	}
	if d["enabled"] != true {
		t.Fatalf("default prefs = %v", d)
	}
	status, _ = doJSON(t, http.MethodPut,
		env.server.URL+"/api/users/me/notification-prefs", token,
		map[string]any{"enabled": false, "events": []string{"download"}})
	if status != http.StatusOK {
		t.Fatalf("put prefs status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet,
		env.server.URL+"/api/users/me/notification-prefs", token, nil)
	if d["enabled"] != false || len(d["events"].([]any)) != 1 {
		t.Fatalf("prefs after save = %v", d)
	}
}

func TestFilterableFeed(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Feed Movie", ".mp4")
	seriesID, _ := env.insertSeriesVideo(t, libID, "Feed Show", 1, 1)

	// favorite + rating + subscription
	if status, _ := doJSON(t, http.MethodPost,
		env.server.URL+"/api/users/me/favorites", token,
		map[string]any{"video_id": videoID.String()}); status != http.StatusCreated {
		t.Fatalf("favorite status = %d", status)
	}
	if status, _ := doJSON(t, http.MethodPut,
		env.server.URL+"/api/videos/"+videoID.String()+"/rating", token,
		map[string]any{"stars": 5}); status != http.StatusOK {
		t.Fatalf("rating status = %d", status)
	}
	if status, _ := doJSON(t, http.MethodPut,
		env.server.URL+"/api/series/"+seriesID.String()+"/subscribe", token, nil); status != http.StatusOK {
		t.Fatalf("subscribe status = %d", status)
	}

	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/feed?type=rating", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) == 0 {
		t.Fatalf("rating feed status = %d body = %v", status, d)
	}
	first := d["items"].([]any)[0].(map[string]any)
	if first["kind"] != "rating" {
		t.Fatalf("filtered feed kind = %v", first["kind"])
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/feed?type=subscription", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) == 0 {
		t.Fatalf("subscription feed status = %d body = %v", status, d)
	}
	sub := d["items"].([]any)[0].(map[string]any)
	if sub["series_name"] != "Feed Show" {
		t.Fatalf("subscription feed item = %v", sub)
	}
}
