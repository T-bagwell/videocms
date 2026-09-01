package media

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	subs := []HLSSubtitle{
		{ID: "a", Name: "English", Active: true},
		{ID: "b", Name: "中文"},
	}
	master := string(buildMaster(renditionPlan(1920, 1080), subs))
	for _, want := range []string{
		"#EXT-X-STREAM-INF:BANDWIDTH=2500000",
		"#EXT-X-STREAM-INF:BANDWIDTH=1500000",
		"v1280/index.m3u8",
		"v854/index.m3u8",
		"#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"English\",DEFAULT=YES,AUTOSELECT=YES,URI=\"subs/a/playlist.m3u8\"",
		"#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"中文\",DEFAULT=NO,AUTOSELECT=YES,URI=\"subs/b/playlist.m3u8\"",
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master playlist missing %q:\n%s", want, master)
		}
	}

	plain := string(buildMaster(renditionPlan(1920, 1080), nil))
	if strings.Contains(plain, "SUBTITLES") {
		t.Error("master playlist should not reference subtitles when none exist")
	}
}

func TestBuildMasterWithAudio(t *testing.T) {
	audios := []HLSAudio{
		{Index: 1, Name: "English"},
		{Index: 2, Name: ""},
	}
	master := string(buildMaster(renditionPlan(1920, 1080), nil, audios...))
	for _, want := range []string{
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"English\",DEFAULT=YES,AUTOSELECT=YES,URI=\"a1/index.m3u8\"",
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"Audio 2\",DEFAULT=NO,AUTOSELECT=YES,URI=\"a2/index.m3u8\"",
		"#EXT-X-STREAM-INF:BANDWIDTH=2500000,AVERAGE-BANDWIDTH=2000000,RESOLUTION=1280x720,AUDIO=\"audio\"",
		"v1280/index.m3u8",
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master playlist missing %q:\n%s", want, master)
		}
	}

	plain := string(buildMaster(renditionPlan(1920, 1080), nil))
	if strings.Contains(plain, "TYPE=AUDIO") || strings.Contains(plain, "AUDIO=\"audio\"") {
		t.Error("master playlist should not advertise an audio group when none exists")
	}
}

func TestSessionFileAllowsAudioTracks(t *testing.T) {
	dir := t.TempDir()
	m := NewHLSManager(dir, "ffmpeg")
	id := uuid.New()
	sessionDir := m.sessionDir(id)
	if err := os.MkdirAll(filepath.Join(sessionDir, "a1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "a1", "index.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "a1", "seg_00001.ts"), []byte("ts"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"a1/index.m3u8", "a1/seg_00001.ts"} {
		if _, ok := m.SessionFile(id, name); !ok {
			t.Errorf("SessionFile(%q) should be allowed", name)
		}
	}
	if _, ok := m.SessionFile(id, "a1/../master.m3u8"); ok {
		t.Error("SessionFile must reject path traversal via audio dir")
	}
	if _, ok := m.SessionFile(id, "a1/other.ts"); ok {
		t.Error("SessionFile must reject non-segment files")
	}
}

func TestHWEncodeOpts(t *testing.T) {
	cases := []struct {
		accel string
		want  []string
		err   bool
	}{
		{"", []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23"}, false},
		{"software", []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23"}, false},
		{"libx264", []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23"}, false},
		{"videotoolbox", []string{"-c:v", "h264_videotoolbox", "-b:v", "3000k", "-allow_sw", "1"}, false},
		{"nvenc", []string{"-c:v", "h264_nvenc", "-preset", "p4", "-b:v", "3000k"}, false},
		{"qsv", []string{"-c:v", "h264_qsv", "-b:v", "3000k"}, false},
		{"bogus", nil, true},
	}
	for i, c := range cases {
		got, err := hwEncodeOpts(c.accel)
		if c.err {
			if err == nil {
				t.Errorf("case %d: hwEncodeOpts(%q) should error", i, c.accel)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %d: hwEncodeOpts(%q) error: %v", i, c.accel, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("case %d: hwEncodeOpts(%q) = %v, want %v", i, c.accel, got, c.want)
		}
	}
}

func TestHWEncodePlan(t *testing.T) {
	vaapi, err := hwEncodePlan("vaapi", "/dev/dri/renderD129", false)
	if err != nil {
		t.Fatalf("vaapi plan: %v", err)
	}
	if got := strings.Join(vaapi.global, " "); !strings.Contains(got, "-vaapi_device /dev/dri/renderD129") ||
		!strings.Contains(got, "-hwaccel vaapi") {
		t.Fatalf("vaapi global = %q", got)
	}
	if got := strings.Join(vaapi.codec, " "); got != "-c:v h264_vaapi -b:v 3000k" {
		t.Fatalf("vaapi codec = %q", got)
	}
	if !strings.Contains(vaapi.scale, "hwupload,scale_vaapi=w=") {
		t.Fatalf("vaapi scale = %q", vaapi.scale)
	}

	tone, err := hwEncodePlan("nvenc", "", true)
	if err != nil {
		t.Fatalf("tone plan: %v", err)
	}
	if !strings.Contains(tone.scale, "tonemap=hable") || !strings.Contains(tone.scale, "scale=") {
		t.Fatalf("tone scale = %q", tone.scale)
	}
	plain, err := hwEncodePlan("", "", false)
	if err != nil {
		t.Fatalf("software plan: %v", err)
	}
	if strings.Contains(plain.scale, "tonemap") {
		t.Fatalf("software scale should not tonemap: %q", plain.scale)
	}

	if _, err := hwEncodePlan("bogus", "", false); err == nil {
		t.Fatal("bogus accel should error")
	}
}

func TestSoftwareEncodePlanVCodecs(t *testing.T) {
	cases := []struct {
		vcodec string
		want   []string
	}{
		{"", []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23"}},
		{"libx265", []string{"-c:v", "libx265", "-preset", "veryfast", "-crf", "25", "-tag:v", "hvc1"}},
		{"libsvtav1", []string{"-c:v", "libsvtav1", "-preset", "8", "-crf", "32"}},
		{"libvpx-vp9", []string{"-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "8", "-crf", "34", "-b:v", "0"}},
	}
	for _, c := range cases {
		plan, err := hwEncodePlanFull("", "", c.vcodec, false, false, false)
		if err != nil {
			t.Fatalf("plan for %q: %v", c.vcodec, err)
		}
		if !reflect.DeepEqual(plan.codec, c.want) {
			t.Errorf("codec for %q = %v, want %v", c.vcodec, plan.codec, c.want)
		}
		if strings.Contains(plan.scale, "tonemap") {
			t.Errorf("scale for %q should not tonemap: %q", c.vcodec, plan.scale)
		}
	}
	if plan, err := hwEncodePlanFull("", "", "bogus", false, false, false); err != nil ||
		!strings.Contains(strings.Join(plan.codec, " "), "libx264") {
		t.Errorf("unknown vcodec should fall back to libx264, got %v (%v)", plan.codec, err)
	}
}

func TestHDRPassthroughPlan(t *testing.T) {
	// 10-bit encoders carry the HDR signal through.
	plan, err := hwEncodePlanFull("", "", "libsvtav1", false, true, true)
	if err != nil {
		t.Fatalf("svtav1 hdr plan: %v", err)
	}
	codec := strings.Join(plan.codec, " ")
	if !strings.Contains(codec, "yuv420p10le") || !strings.Contains(codec, "smpte2084") {
		t.Errorf("hdr passthrough codec = %q, want 10-bit + bt2020 metadata", codec)
	}
	if strings.Contains(plan.scale, "tonemap") {
		t.Errorf("hdr passthrough should not tonemap: %q", plan.scale)
	}

	// 8-bit encoders fall back to tone mapping.
	plan, err = hwEncodePlanFull("", "", "libx264", false, true, true)
	if err != nil {
		t.Fatalf("x264 hdr plan: %v", err)
	}
	if !strings.Contains(plan.scale, "tonemap=hable") {
		t.Errorf("x264 hdr fallback scale = %q, want tone mapping", plan.scale)
	}
	if strings.Contains(strings.Join(plan.codec, " "), "yuv420p10le") {
		t.Error("x264 plan must stay 8-bit")
	}

	// Passthrough disabled: plain SDR transcode.
	plan, err = hwEncodePlanFull("", "", "libvpx-vp9", false, false, true)
	if err != nil {
		t.Fatalf("vp9 non-passthrough plan: %v", err)
	}
	if strings.Contains(strings.Join(plan.codec, " "), "yuv420p10le") {
		t.Error("passthrough disabled should not request 10-bit output")
	}

	// Explicit tone mapping wins over passthrough.
	plan, err = hwEncodePlanFull("", "", "libsvtav1", true, true, true)
	if err != nil {
		t.Fatalf("svtav1 tonemap plan: %v", err)
	}
	if !strings.Contains(plan.scale, "tonemap") {
		t.Errorf("explicit tonemap scale = %q", plan.scale)
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
