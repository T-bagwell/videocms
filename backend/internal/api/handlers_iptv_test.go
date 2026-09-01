package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIPTVChannelsAndEPG(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	m3u := "#EXTM3U\n" +
		"#EXTINF:-1 tvg-id=\"cnn.us\" tvg-name=\"CNN\" tvg-logo=\"http://logo/cnn.png\" group-title=\"News\",CNN HD\n" +
		"http://example.com/cnn.m3u8\n" +
		"#EXTINF:-1 tvg-id=\"bbc.uk\",BBC One\n" +
		"http://example.com/bbc.m3u8\n"
	m3uSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, m3u)
	}))
	defer m3uSrv.Close()

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/iptv/import", token,
		map[string]any{"url": m3uSrv.URL})
	if status != http.StatusOK {
		t.Fatalf("m3u import status = %d body = %v", status, d)
	}
	if int(d["imported"].(float64)) != 2 {
		t.Fatalf("imported = %v, want 2", d["imported"])
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/iptv/channels", token, nil)
	if status != http.StatusOK {
		t.Fatalf("channels status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("channel count = %d, want 2", len(items))
	}

	// M3U output requires a token and includes imported entries.
	status, _ = doJSON(t, http.MethodGet, env.server.URL+"/api/iptv/channels.m3u", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("m3u without token status = %d, want 401", status)
	}
	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/api/iptv/channels.m3u?token="+token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 64<<10)
	n, _ := resp.Body.Read(out)
	_ = resp.Body.Close()
	body := string(out[:n])
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "tvg-id=\"cnn.us\"") ||
		!strings.Contains(body, "http://example.com/bbc.m3u8") {
		t.Fatalf("m3u output = %q", body)
	}

	// Library-generated channel appears with a stream URL.
	libID := env.insertLibrary(t, false)
	env.insertVideo(t, libID, "Movie A", ".mp4")
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/iptv/library-channel", token,
		map[string]any{"library_id": libID.String(), "name": "My TV", "tvg_id": "my.tv"})
	if status != http.StatusCreated {
		t.Fatalf("library channel status = %d body = %v", status, d)
	}
	channelID, _ := uuid.Parse(d["id"].(string))
	req, _ = http.NewRequest(http.MethodGet, env.server.URL+"/api/iptv/channels.m3u?token="+token, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out = make([]byte, 64<<10)
	n, _ = resp.Body.Read(out)
	_ = resp.Body.Close()
	body = string(out[:n])
	wantURL := fmt.Sprintf("/api/iptv/library/%s/stream?token=", channelID)
	if !strings.Contains(body, wantURL) {
		t.Fatalf("library channel URL missing from m3u:\n%s", body)
	}

	// EPG import + XML output.
	xmltv := `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="cnn.us"><display-name>CNN</display-name></channel>
  <programme channel="cnn.us" start="20260901120000 +0000" end="20260901130000 +0000">
    <title>News Hour</title><desc>Top stories</desc>
  </programme>
</tv>`
	epgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, xmltv)
	}))
	defer epgSrv.Close()
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/iptv/epg/import", token,
		map[string]any{"url": epgSrv.URL})
	if status != http.StatusOK {
		t.Fatalf("epg import status = %d body = %v", status, d)
	}
	req, _ = http.NewRequest(http.MethodGet, env.server.URL+"/api/iptv/epg.xml?token="+token, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out = make([]byte, 128<<10)
	n, _ = resp.Body.Read(out)
	_ = resp.Body.Close()
	body = string(out[:n])
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "<title>News Hour</title>") {
		t.Fatalf("epg output = %q", body)
	}

	// Library channel stream starts and serves MPEG-TS.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet,
		env.server.URL+"/api/iptv/library/"+channelID.String()+"/stream?token="+token, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "video/mp2t") {
		t.Fatalf("library stream status = %d ct = %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	chunk := make([]byte, 8192)
	_, _ = resp.Body.Read(chunk)
}
