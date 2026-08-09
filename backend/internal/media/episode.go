package media

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	seasonEpRe    = regexp.MustCompile(`(?i)\bs(\d{1,2})e(\d{1,3})\b`)
	episodeRe     = regexp.MustCompile(`(?i)\bep(?:isode)?[.\-_ ]?(\d{1,3})\b`)
	bareEpisodeRe = regexp.MustCompile(`(?i)\be(\d{1,3})\b`)
	cnEpisodeRe   = regexp.MustCompile(`第\s*(\d{1,3})\s*[集話话]`)
	trailingRe    = regexp.MustCompile(`^(.*?)[\s]*[\(\[](\d{1,3})[\)\]]\s*$`)
	bareTrailing  = regexp.MustCompile(`^(.*?)[\s\-_.]+(\d{1,3})\s*$`)
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

func cleanSeriesName(name string) string {
	name = strings.TrimSpace(name)
	// drop trailing separators/punctuation that preceded the episode number
	name = strings.TrimRight(name, " \t._-–—:：|")
	return name
}
