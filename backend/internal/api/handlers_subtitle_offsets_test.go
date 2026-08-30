package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestShiftVTT(t *testing.T) {
	in := []byte("WEBVTT\n\n00:00:01.000 --> 00:00:03.500 align:start\nHello\n\n00:00:10.000 --> 00:00:12.000\nWorld\n")

	// +1.5s
	out := string(shiftVTT(in, 1500))
	for _, want := range []string{
		"00:00:02.500 --> 00:00:05.000 align:start",
		"00:00:11.500 --> 00:00:13.500",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("shifted VTT missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "00:00:01.000") {
		t.Errorf("shifted VTT still contains the original timestamp:\n%s", out)
	}

	// −3s clamps the start at zero instead of producing a negative time
	out = string(shiftVTT(in, -3000))
	if !strings.Contains(out, "00:00:00.000 --> 00:00:00.500 align:start") {
		t.Errorf("negative offset did not clamp to zero:\n%s", out)
	}

	// zero offset returns the input unchanged
	if got := string(shiftVTT(in, 0)); got != string(in) {
		t.Error("zero offset must return the input byte-for-byte")
	}
}

func TestSubtitleOffsetLifecycle(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "offset-test", ".mkv")
	base := env.server.URL + "/api/users/me/subtitle-offset"

	status, d := doJSON(t, "GET", base+"?video_id="+videoID.String(), token, nil)
	if status != http.StatusOK || d["offset_ms"].(float64) != 0 {
		t.Fatalf("initial offset status=%d body=%v, want 0", status, d)
	}

	status, d = doJSON(t, "PUT", base, token,
		map[string]any{"video_id": videoID.String(), "offset_ms": 1500})
	if status != http.StatusOK || d["offset_ms"].(float64) != 1500 {
		t.Fatalf("set offset status=%d body=%v", status, d)
	}

	status, d = doJSON(t, "GET", base+"?video_id="+videoID.String(), token, nil)
	if status != http.StatusOK || d["offset_ms"].(float64) != 1500 {
		t.Fatalf("read offset status=%d body=%v", status, d)
	}

	// values beyond ±5 minutes are clamped
	status, d = doJSON(t, "PUT", base, token,
		map[string]any{"video_id": videoID.String(), "offset_ms": 700000})
	if status != http.StatusOK || d["offset_ms"].(float64) != float64(maxSubtitleOffsetMs) {
		t.Fatalf("clamp status=%d body=%v", status, d)
	}

	status, d = doJSON(t, "DELETE", base+"?video_id="+videoID.String(), token, nil)
	if status != http.StatusOK || d["offset_ms"].(float64) != 0 {
		t.Fatalf("clear status=%d body=%v", status, d)
	}

	// unknown video is rejected
	status, _ = doJSON(t, "PUT", base, token,
		map[string]any{"video_id": "00000000-0000-0000-0000-000000000000", "offset_ms": 100})
	if status != http.StatusBadRequest {
		t.Fatalf("unknown video status=%d, want 400", status)
	}
}
