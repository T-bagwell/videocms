package api

import (
	"net/http"
	"testing"
)

func TestPlaybackSpeedSavedPerUser(t *testing.T) {
	env := newIntegrationEnv(t)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Audiobook", ".m4b")
	token := loginAdmin(t, env)

	save := func(rate any) int {
		t.Helper()
		body := map[string]any{
			"video_id":     videoID.String(),
			"position_sec": 10,
			"duration_sec": 100,
		}
		if rate != nil {
			body["playback_rate"] = rate
		}
		status, d := doJSON(t, http.MethodPut,
			env.server.URL+"/api/users/me/progress", token, body)
		if status != http.StatusOK {
			t.Fatalf("save progress status = %d, body = %v", status, d)
		}
		return status
	}
	load := func() float64 {
		t.Helper()
		status, d := doJSON(t, http.MethodGet,
			env.server.URL+"/api/users/me/playback-speed/"+videoID.String(), token, nil)
		if status != http.StatusOK {
			t.Fatalf("load speed status = %d, body = %v", status, d)
		}
		r, _ := d["playback_rate"].(float64)
		return r
	}

	if load() != 1 {
		t.Fatalf("default playback rate = %v, want 1", load())
	}
	save(1.75)
	if got := load(); got != 1.75 {
		t.Fatalf("saved rate = %v, want 1.75", got)
	}
	// Saves without a rate keep the remembered speed.
	save(nil)
	if got := load(); got != 1.75 {
		t.Fatalf("rate after plain save = %v, want 1.75", got)
	}
	// Out-of-range values clamp to the audiobook-safe bounds.
	save(9)
	if got := load(); got != 3 {
		t.Errorf("rate 9 clamped to %v, want 3", got)
	}
	save(0.1)
	if got := load(); got != 0.25 {
		t.Errorf("rate 0.1 clamped to %v, want 0.25", got)
	}
}
