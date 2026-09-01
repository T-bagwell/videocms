package media

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// movieVersionPatterns matches quality/edition markers commonly found in
// multi-version movie filenames. The same film's files (1080p / 4K / extended
// cut) differ only in these markers, so stripping them yields a grouping key.
var (
	versionQualityRe = regexp.MustCompile(`(?i)\b(4k|uhd|2160p|1440p|1080p|1080|720p|576p|480p|360p|sd|hd|web-?dl|webrip|blu-?ray|hdtv|remux|hdrip|bdrip|amzn|netflix|disney\+?|x264|x265|h\.?264|h\.?265|hevc|av1|hdr10\+?|hdr|dv|dolby\s*vision|10bit|8bit|imax|multi-?audio)\b`)

	versionEditionPatterns = []struct {
		re    *regexp.Regexp
		label string
	}{
		{regexp.MustCompile(`(?i)\bextended cut\b|\bextended\b`), "Extended Cut"},
		{regexp.MustCompile(`(?i)\bdirector'?s? cut\b|\bdirectors cut\b`), "Director's Cut"},
		{regexp.MustCompile(`(?i)\btheatrical\b`), "Theatrical"},
		{regexp.MustCompile(`(?i)\buncut\b`), "Uncut"},
		{regexp.MustCompile(`(?i)\bunrated\b`), "Unrated"},
		{regexp.MustCompile(`(?i)\bremastered\b`), "Remastered"},
		{regexp.MustCompile(`(?i)\bspecial edition\b`), "Special Edition"},
		{regexp.MustCompile(`(?i)\blimited edition\b`), "Limited Edition"},
		{regexp.MustCompile(`(?i)\bcollector'?s? edition\b`), "Collector's Edition"},
		{regexp.MustCompile(`(?i)\banniversary edition\b`), "Anniversary Edition"},
	}
)

// ParseMovieVersion extracts the version-grouping key, display label and
// quality rank for a movie file. The key is derived from the cleaned title so
// that renamed metadata still groups correctly; label and rank come from the
// original filename (resolution/edition markers) and the probed dimensions.
func ParseMovieVersion(filename, title string, width, height int) (key, label string, rank int) {
	key = movieVersionKey(title)
	if key == "" {
		return "", "", 0
	}
	label = movieVersionLabel(filename, width, height)
	rank = movieVersionRank(filename, width, height)
	return key, label, rank
}

// TitleWithYear appends the release year to a grouping title in the canonical
// "Title (1999)" form. Scan-time titles already have the year stripped, while
// manually edited metadata may or may not include it; the helper keeps the
// result stable for both.
func TitleWithYear(title string, year int) string {
	if year <= 0 {
		return title
	}
	if yearRe.MatchString(title) {
		return title
	}
	return fmt.Sprintf("%s (%d)", title, year)
}

// movieVersionKey normalizes a title into a stable grouping key by dropping
// year, quality and edition markers, so "The Matrix (1999) 1080p.mkv" and
// "The Matrix (1999) 4K Extended Cut.mkv" share the key "the matrix 1999".
func movieVersionKey(title string) string {
	s := strings.TrimSpace(title)
	if s == "" {
		return ""
	}
	if m := yearRe.FindStringSubmatch(s); m != nil {
		year := m[1]
		s = yearRe.ReplaceAllString(s, " ")
		s = strings.TrimSpace(s) + " " + year
	}
	s = versionQualityRe.ReplaceAllString(s, " ")
	for _, p := range versionEditionPatterns {
		s = p.re.ReplaceAllString(s, " ")
	}
	s = spaceRe.ReplaceAllString(strings.TrimSpace(s), " ")
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return ""
	}
	return key
}

// movieVersionLabel renders a human-readable version label such as
// "4K" or "1080p Extended Cut" from filename markers, falling back to the
// probed resolution tier.
func movieVersionLabel(filename string, width, height int) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	var parts []string
	if m := resolutionMarkerRe.FindStringSubmatch(base); m != nil {
		if l := normalizeResolution(m[1]); l != "" {
			parts = append(parts, l)
		}
	}
	for _, p := range versionEditionPatterns {
		if p.re.MatchString(base) {
			parts = append(parts, p.label)
			break
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if height > 0 {
		switch {
		case height >= 2000:
			return "4K"
		case height >= 1400:
			return "1440p"
		case height >= 1000:
			return "1080p"
		case height >= 700:
			return "720p"
		default:
			return "SD"
		}
	}
	return ""
}

// movieVersionRank scores a file for best-version selection: resolution tier
// dominates, with a small bonus for alternative editions (extended, director's
// cut, remastered...).
func movieVersionRank(filename string, width, height int) int {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	tier := 0
	if m := resolutionMarkerRe.FindStringSubmatch(base); m != nil {
		tier = resolutionTier(m[1])
	}
	if tier == 0 && height > 0 {
		switch {
		case height >= 2000:
			tier = 5
		case height >= 1400:
			tier = 4
		case height >= 1000:
			tier = 3
		case height >= 700:
			tier = 2
		default:
			tier = 1
		}
	}
	rank := tier * 10
	for _, p := range versionEditionPatterns {
		if p.re.MatchString(base) {
			rank++
			break
		}
	}
	return rank
}

var resolutionMarkerRe = regexp.MustCompile(`(?i)\b(4k|uhd|2160p|1440p|1080p|1080|720p|576p|480p|360p|sd)\b`)

func normalizeResolution(marker string) string {
	switch strings.ToLower(marker) {
	case "4k", "uhd", "2160p":
		return "4K"
	case "1440p":
		return "1440p"
	case "1080p", "1080":
		return "1080p"
	case "720p":
		return "720p"
	case "576p":
		return "576p"
	case "480p":
		return "480p"
	case "360p", "sd":
		return "SD"
	}
	return ""
}

func resolutionTier(marker string) int {
	switch strings.ToLower(marker) {
	case "4k", "uhd", "2160p":
		return 5
	case "1440p":
		return 4
	case "1080p", "1080":
		return 3
	case "720p":
		return 2
	case "576p", "480p", "360p", "sd":
		return 1
	}
	return 0
}
