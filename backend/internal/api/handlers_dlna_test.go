package api

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"videocms/backend/internal/config"
)

func TestDLNABrowseAndStream(t *testing.T) {
	env := newIntegrationEnv(t, func(c *config.Config) { c.DLNAEnabled = true })
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertRealVideo(t, libID, "DLNA Movie", makeTestVideo(t))

	res, err := http.Get(env.server.URL + "/dlna/device.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("device.xml status = %d", res.StatusCode)
	}
	var body bytes.Buffer
	_, _ = body.ReadFrom(res.Body)
	xml := body.String()
	if !strings.Contains(xml, "urn:schemas-upnp-org:device:MediaServer:1") ||
		!strings.Contains(xml, "ContentDirectory") {
		t.Fatalf("device.xml missing DLNA bits: %s", xml[:minInt(400, len(xml))])
	}

	// Root browse lists libraries.
	res, err = http.Get(env.server.URL + "/dlna/content/0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body.Reset()
	_, _ = body.ReadFrom(res.Body)
	root := body.String()
	if !strings.Contains(root, `id="lib-`+libID.String()+`"`) {
		t.Fatalf("root browse missing library: %s", root)
	}

	// Library browse lists the video with a stream res.
	res, err = http.Get(env.server.URL + "/dlna/content/lib-" + libID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body.Reset()
	_, _ = body.ReadFrom(res.Body)
	libXML := body.String()
	if !strings.Contains(libXML, "DLNA Movie") ||
		!strings.Contains(libXML, "/dlna/video/"+videoID.String()+"/stream") {
		t.Fatalf("library browse missing video: %s", libXML)
	}

	// SOAP control endpoint works too.
	soap := `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><ObjectID>0</ObjectID><BrowseFlag>BrowseDirectChildren</BrowseFlag><Filter>*</Filter><StartingIndex>0</StartingIndex><RequestedCount>100</RequestedCount><SortCriteria></SortCriteria></u:Browse></s:Body></s:Envelope>`
	res, err = http.Post(env.server.URL+"/dlna/control/ContentDirectory", "text/xml; charset=\"utf-8\"", strings.NewReader(soap))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body.Reset()
	_, _ = body.ReadFrom(res.Body)
	if res.StatusCode != http.StatusOK || !strings.Contains(body.String(), "BrowseResponse") {
		t.Fatalf("SOAP browse status=%d body=%s", res.StatusCode, body.String())
	}

	// Stream endpoint serves the file bytes (range-friendly).
	req, _ := http.NewRequest("GET", env.server.URL+"/dlna/video/"+videoID.String()+"/stream", nil)
	req.Header.Set("Range", "bytes=0-15")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusPartialContent && res.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", res.StatusCode)
	}

	// Admin API still requires auth for this video (sanity check).
	status, _ := doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String(), "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated api status = %d", status)
	}
	_ = token
}

func TestDLNAIPRestriction(t *testing.T) {
	env := newIntegrationEnv(t, func(c *config.Config) {
		c.DLNAEnabled = true
		c.DLNAAllowedIPs = []string{"10.0.0.0/8"}
	})
	res, err := http.Get(env.server.URL + "/dlna/device.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("device.xml status = %d, want 403 (client not in allowlist)", res.StatusCode)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
