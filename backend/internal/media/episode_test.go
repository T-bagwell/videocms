package media

import (
	"strconv"
	"testing"
)

func TestParseEpisode(t *testing.T) {
	cases := []struct {
		title      string
		wantSeries string
		wantSeason int
		wantEp     int
	}{
		{"1 (4)", "1", 0, 4},
		{"1 (28)", "1", 0, 28},
		{"星际迷航 S01E01", "星际迷航", 1, 1},
		{"城市猎人 第1集", "城市猎人", 0, 1},
		{"封神榜01女娲宫风波", "封神榜", 0, 1},
		{"封神榜20西岐山雪攻", "封神榜", 0, 20},
		{"Show E03", "Show", 0, 3},
		{"SSIS-698", "SSIS", 0, 698},
		{"星际穿越 Interstellar 2014", "", 0, 0},
		{"2024 2 12 利哥探花 黑丝", "", 0, 0},
		{"The Room 101", "The Room", 0, 101},
	}
	for _, c := range cases {
		gotSeries, gotSeason, gotEp := parseEpisode(c.title)
		if gotSeries != c.wantSeries || gotSeason != c.wantSeason || gotEp != c.wantEp {
			t.Errorf("%q: got (%q,%d,%d) want (%q,%d,%d)",
				c.title, gotSeries, gotSeason, gotEp, c.wantSeries, c.wantSeason, c.wantEp)
		}
	}
}

func TestMidNumberRe(t *testing.T) {
	m := midNumberRe.FindStringSubmatchIndex("封神榜01女娲宫风波")
	t.Logf("indices: %v", m)
	if m == nil {
		t.Fatalf("no match")
	}
	prefix := "封神榜01女娲宫风波"[:m[3]]
	t.Logf("prefix=%q hasLetter=%v", prefix, hasLetter(prefix))
	s, se, ep := parseEpisode("封神榜01女娲宫风波")
	t.Logf("parseEpisode -> (%q,%d,%d)", s, se, ep)
}

func TestBareEpisodeRe(t *testing.T) {
	for title, want := range map[string]int{
		"01": 1, "1": 1, "30": 30, "120": 120,
		"2024": 0, "1a": 0, "S01E01": 0, "": 0,
	} {
		m := bareNumberRe.FindStringSubmatch(title)
		got := 0
		if m != nil {
			got, _ = strconv.Atoi(m[1])
		}
		if got != want {
			t.Errorf("%q: got %d want %d", title, got, want)
		}
	}
}

func TestFallbackSeriesName(t *testing.T) {
	cases := []struct {
		libName, libPath, filePath string
		want                       string
	}{
		// files directly in the library root inherit the library name
		{"胜者为王2", "/media/lq/WD_SSD/胜者为王2全集 无水印版本 1080P",
			"/media/lq/WD_SSD/胜者为王2全集 无水印版本 1080P/01.mkv", "胜者为王2"},
		// top-level directory names the series
		{"媒体库", "/media/lq/WD_SSD", "/media/lq/WD_SSD/胜者为王2/02.mkv", "胜者为王2"},
		// nested season folders still resolve to the top-level series directory
		{"媒体库", "/media/lq/WD_SSD",
			"/media/lq/WD_SSD/星际迷航/Season 1/03.mkv", "星际迷航"},
		// files outside the library root are rejected
		{"媒体库", "/media/lq/WD_SSD", "/other/04.mkv", ""},
	}
	for _, c := range cases {
		if got := fallbackSeriesName(c.libName, c.libPath, c.filePath); got != c.want {
			t.Errorf("fallbackSeriesName(%q,%q,%q) = %q, want %q",
				c.libName, c.libPath, c.filePath, got, c.want)
		}
	}
}
