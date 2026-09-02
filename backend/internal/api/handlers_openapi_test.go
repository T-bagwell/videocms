package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestOpenAPIDocument(t *testing.T) {
	env := newIntegrationEnv(t)
	resp, err := http.Get(env.server.URL + "/api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openapi status = %d", resp.StatusCode)
	}
	var doc struct {
		OpenAPI string                 `json:"openapi"`
		Paths   map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("invalid openapi json: %v", err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("openapi version = %q", doc.OpenAPI)
	}
	if len(doc.Paths) < 100 {
		t.Fatalf("paths = %d, want >= 100", len(doc.Paths))
	}
	for _, must := range []string{
		"/api/videos/{id}/versions", "/api/albums", "/api/books/{id}/epub/spine",
		"/api/iptv/channels.m3u", "/api/public/videos/{id}/unlock", "/api/admin/reports",
	} {
		if _, ok := doc.Paths[must]; !ok {
			t.Errorf("missing path %s", must)
		}
	}

	resp2, err := http.Get(env.server.URL + "/api/docs")
	if err != nil {
		t.Fatal(err)
	}
	docs, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK || !containsStr(string(docs), "swagger-ui") {
		t.Fatalf("docs page status = %d", resp2.StatusCode)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
