package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestShareCustomization(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Share", ".mkv")

	status, d := doJSON(t, "POST", env.server.URL+"/api/videos/"+videoID.String()+"/share", token,
		map[string]any{"hours": 1, "theme": "dark", "custom_title": "My Title", "hide_nav": true})
	if status != http.StatusCreated {
		t.Fatalf("create share status = %d, body = %v", status, d)
	}
	shareToken, _ := d["token"].(string)

	res, err := http.Get(env.server.URL + "/api/share/" + shareToken + "/info")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("info status = %d: %s", res.StatusCode, b)
	}
	var info map[string]any
	_ = json.NewDecoder(res.Body).Decode(&info)
	if info["theme"] != "dark" || info["custom_title"] != "My Title" || info["hide_nav"] != true {
		t.Fatalf("share info customization = %v", info)
	}
}
