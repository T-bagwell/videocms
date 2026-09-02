package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	env := newIntegrationEnv(t)

	// Trigger a couple of requests so counters exist.
	status, _ := doJSON(t, http.MethodGet, env.server.URL+"/api/healthz", "", nil)
	if status != http.StatusOK {
		t.Fatalf("healthz status = %d", status)
	}

	resp, err := http.Get(env.server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d", resp.StatusCode)
	}
	text := string(body)
	for _, want := range []string{
		"videocms_http_requests_total{",
		"videocms_videos_total",
		"videocms_libraries_total",
		"videocms_uptime_seconds",
		"/api/healthz",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q:\n%s", want, text)
		}
	}
}
