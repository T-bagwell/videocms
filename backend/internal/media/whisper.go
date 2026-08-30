package media

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Transcribe runs a whisper.cpp-style CLI and writes a WebVTT transcript to
// outVTT. The command is `-m <model> -f <input> -ovtt -of <outBase>`.
func Transcribe(ctx context.Context, bin, model, input, outVTT string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	outBase := strings.TrimSuffix(outVTT, ".vtt")
	cmd := exec.CommandContext(ctx, bin, "-m", model, "-f", input, "-ovtt", "-of", outBase)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("whisper: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// VTTText extracts the cue text (without timestamps/headers) from a WebVTT
// document, for full-text search.
func VTTText(data []byte) string {
	lines := strings.Split(string(data), "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "WEBVTT") ||
			strings.Contains(line, " --> ") || strings.HasPrefix(line, "NOTE") {
			continue
		}
		b.WriteString(line)
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}
