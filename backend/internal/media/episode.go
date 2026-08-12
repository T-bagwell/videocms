package media

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	seasonEpRe    = regexp.MustCompile(`(?i)\bs(\d{1,2})e(\d{1,3})\b`)
	episodeRe     = regexp.MustCompile(`(?i)\bep(?:isode)?[.\-_ ]?(\d{1,3})\b`)
	bareEpisodeRe = regexp.MustCompile(`(?i)\be(\d{1,3})\b`)
	cnEpisodeRe   = regexp.MustCompile(`第\s*(\d{1,3})\s*[集話话]`)
	midNumberRe   = regexp.MustCompile(`^(.+?[^\d])(\d{2,3})([^\d]|$)`)
	trailingRe    = regexp.MustCompile(`^(.*?)[\s]*[\(\[](\d{1,3})[\)\]]\s*$`)
	bareTrailing  = regexp.MustCompile(`^(.*?)[\s\-_.]+(\d{1,3})\s*$`)
	bareNumberRe  = regexp.MustCompile(`^(\d{1,3})$`)
)

// parseEpisode extracts a (series name, season, episode) tuple from a cleaned
// title. Returns an empty series name when no episode marker is found.
func parseEpisode(title string) (seriesName string, season, episode int) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", 0, 0
	}

	if m := seasonEpRe.FindStringSubmatchIndex(title); m != nil {
		season, _ = strconv.Atoi(title[m[2]:m[3]])
		episode, _ = strconv.Atoi(title[m[4]:m[5]])
		return cleanSeriesName(title[:m[0]]), season, episode
	}
	if m := episodeRe.FindStringSubmatchIndex(title); m != nil {
		episode, _ = strconv.Atoi(title[m[2]:m[3]])
		return cleanSeriesName(title[:m[0]]), 0, episode
	}
	if m := bareEpisodeRe.FindStringSubmatchIndex(title); m != nil {
		episode, _ = strconv.Atoi(title[m[2]:m[3]])
		return cleanSeriesName(title[:m[0]]), 0, episode
	}
	if m := cnEpisodeRe.FindStringSubmatchIndex(title); m != nil {
		episode, _ = strconv.Atoi(title[m[2]:m[3]])
		return cleanSeriesName(title[:m[0]]), 0, episode
	}
	// "ShowName01EpisodeTitle" style: series name, then 2-3 digit episode number
	if m := midNumberRe.FindStringSubmatchIndex(title); m != nil {
		prefix := title[:m[3]]
		// require a letter/CJK character so date-like prefixes (2024 02 12 …) don't match
		if hasLetter(prefix) {
			episode, _ = strconv.Atoi(title[m[4]:m[5]])
			return cleanSeriesName(prefix), 0, episode
		}
	}
	if m := trailingRe.FindStringSubmatchIndex(title); m != nil {
		episode, _ = strconv.Atoi(title[m[4]:m[5]])
		return cleanSeriesName(title[:m[3]]), 0, episode
	}
	if m := bareTrailing.FindStringSubmatchIndex(title); m != nil {
		episode, _ = strconv.Atoi(title[m[4]:m[5]])
		return cleanSeriesName(title[:m[3]]), 0, episode
	}
	return "", 0, 0
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func cleanSeriesName(name string) string {
	name = strings.TrimSpace(name)
	// drop trailing separators/punctuation that preceded the episode number
	name = strings.TrimRight(name, " \t._-–—:：|")
	return name
}

// fallbackSeriesName derives a series name for a video whose filename is a bare
// episode number (e.g. "01.mkv") with no other marker: the top-level directory
// under the library root, or the library name when the file sits directly in
// the library root. Returns "" when the file is outside the library root.
func fallbackSeriesName(libName, libPath, filePath string) string {
	rel, err := filepath.Rel(libPath, filepath.Dir(filePath))
	if err != nil {
		return ""
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	if rel == "." {
		return libName
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return cleanSeriesName(parts[0])
}
