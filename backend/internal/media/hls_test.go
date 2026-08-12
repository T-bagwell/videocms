package media

import (
	"strings"
	"testing"
)

func TestRenditionPlanFiltersBySourceWidth(t *testing.T) {
	plan := renditionPlan(1920, 1080)
	if len(plan) != 4 {
		t.Fatalf("1080p source: got %d renditions, want 4", len(plan))
	}
	if plan[0].Name != "v1280" {
		t.Errorf("largest rendition = %s, want v1280", plan[0].Name)
	}

	plan = renditionPlan(854, 480)
	if len(plan) != 3 {
		t.Fatalf("480p source: got %d renditions, want 3", len(plan))
	}
	if plan[0].Name != "v854" {
		t.Errorf("largest rendition = %s, want v854", plan[0].Name)
	}

	plan = renditionPlan(0, 0)
	if len(plan) != 1 || plan[0].Name != "v1280" {
		t.Fatalf("unknown source: got %v, want single v1280", plan)
	}

	plan = renditionPlan(320, 180)
	if len(plan) != 1 || plan[0].Name != "v426" {
		t.Fatalf("narrow source: got %v, want single v426", plan)
	}
}

func TestRenditionPlanKeepsAspectRatio(t *testing.T) {
	plan := renditionPlan(1920, 1080)
	if plan[0].Height != 720 {
		t.Errorf("1280px rendition height = %d, want 720", plan[0].Height)
	}
	// heights must stay even for x264
	for _, r := range plan {
		if r.Height%2 != 0 {
			t.Errorf("%s height %d is odd", r.Name, r.Height)
		}
	}
}

func TestBuildMaster(t *testing.T) {
	master := string(buildMaster(renditionPlan(1920, 1080), true))
	for _, want := range []string{
		"#EXT-X-STREAM-INF:BANDWIDTH=2500000",
		"#EXT-X-STREAM-INF:BANDWIDTH=1500000",
		"v1280/index.m3u8",
		"v854/index.m3u8",
		"#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"Subtitles\",DEFAULT=YES,AUTOSELECT=YES,URI=\"subs/playlist.m3u8\"",
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master playlist missing %q:\n%s", want, master)
		}
	}

	plain := string(buildMaster(renditionPlan(1920, 1080), false))
	if strings.Contains(plain, "SUBTITLES") {
		t.Error("master playlist should not reference subtitles when none exist")
	}
}

func TestIsImageSubtitleCodec(t *testing.T) {
	for _, c := range []string{"hdmv_pgs_subtitle", "DVD_SUBTITLE", "dvb_subtitle"} {
		if !IsImageSubtitleCodec(c) {
			t.Errorf("%q should be treated as image-based", c)
		}
	}
	for _, c := range []string{"subrip", "ass", "mov_text", "webvtt"} {
		if IsImageSubtitleCodec(c) {
			t.Errorf("%q should be treated as text", c)
		}
	}
}

func TestFirstTextSubtitle(t *testing.T) {
	streams := []SubtitleStream{
		{Index: 2, Codec: "hdmv_pgs_subtitle"},
		{Index: 3, Codec: "subrip"},
	}
	if got := FirstTextSubtitle(streams); got != 3 {
		t.Errorf("FirstTextSubtitle = %d, want 3", got)
	}
	if got := FirstTextSubtitle(nil); got != -1 {
		t.Errorf("FirstTextSubtitle(nil) = %d, want -1", got)
	}
}
