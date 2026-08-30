package api

import (
	"net/http"
	"testing"
)

func TestWatchRoomLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "watch", ".mkv")
	base := env.server.URL + "/api/watch/rooms"

	status, d := doJSON(t, "POST", base, token, map[string]any{"video_id": videoID.String()})
	if status != http.StatusCreated {
		t.Fatalf("create room status = %d, body = %v", status, d)
	}
	roomID, _ := d["id"].(string)
	roomToken, _ := d["token"].(string)
	if roomID == "" || roomToken == "" {
		t.Fatalf("create room missing id/token: %v", d)
	}

	status, d = doJSON(t, "GET", base+"/"+roomID+"?token="+roomToken, token, nil)
	if status != http.StatusOK || d["video_title"] != "watch" {
		t.Fatalf("get room status = %d, body = %v", status, d)
	}

	status, _ = doJSON(t, "GET", base+"/"+roomID+"?token=wrong", token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("bad token status = %d, want 403", status)
	}

	status, d = doJSON(t, "PUT", base+"/"+roomID, token,
		map[string]any{"token": roomToken, "playing": true, "position_sec": 42})
	if status != http.StatusOK {
		t.Fatalf("update status = %d, body = %v", status, d)
	}
	if d["playing"] != true || d["position_sec"].(float64) != 42 {
		t.Fatalf("updated state = %v", d)
	}

	status, d = doJSON(t, "POST", base+"/"+roomID+"/join", token, map[string]any{"token": roomToken})
	if status != http.StatusOK || d["playing"] != true {
		t.Fatalf("join status = %d, body = %v", status, d)
	}

	status, _ = doJSON(t, "POST", base, token,
		map[string]any{"video_id": "00000000-0000-0000-0000-000000000000"})
	if status != http.StatusBadRequest {
		t.Fatalf("unknown video status = %d, want 400", status)
	}
}
