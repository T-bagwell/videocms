package media

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// AnalyzeTags runs an external AI tagging tool (e.g. a CLIP-based script) with
// the media path as its single argument and reads one tag per stdout line.
func AnalyzeTags(ctx context.Context, bin, input string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, input).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ai tagger: %w: %s", err, strings.TrimSpace(string(out)))
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		tag := strings.ToLower(strings.TrimSpace(line))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) >= 20 {
			break
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("ai tagger produced no tags")
	}
	return tags, nil
}
