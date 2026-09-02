package media

import "testing"

func TestSafeHTTPURL(t *testing.T) {
	if _, err := SafeHTTPURL("http://example.com/video.m3u8", false); err != nil {
		t.Fatalf("public url rejected: %v", err)
	}
	for _, raw := range []string{
		"http://127.0.0.1/x",
		"http://[::1]/x",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/x",
		"http://192.168.1.10/x",
		"http://[::ffff:127.0.0.1]/x",
		"ftp://example.com/x",
		"not a url",
		"",
	} {
		if _, err := SafeHTTPURL(raw, false); err == nil {
			t.Errorf("unsafe url accepted: %q", raw)
		}
	}
	// Local fetches are allowed explicitly (tests, trusted integrations).
	if _, err := SafeHTTPURL("http://127.0.0.1:8080/x", true); err != nil {
		t.Fatalf("local url rejected with allowLocal: %v", err)
	}
}
