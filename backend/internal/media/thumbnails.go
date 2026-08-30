package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	thumbIntervalSec = 10
	thumbCellWidth   = 160
	thumbCellHeight  = 90
	thumbMaxFrames   = 120
)

// ThumbnailsMeta describes a generated thumbnail frame set.
type ThumbnailsMeta struct {
	Count       int `json:"count"`
	IntervalSec int `json:"interval_sec"`
	Width       int `json:"width"`
	Height      int `json:"height"`
}

// GenerateThumbnails extracts evenly spaced frames (every 10s, capped at 120)
// from input into dir as frame_001.jpg, frame_002.jpg, … and returns the
// number of frames actually written.
func GenerateThumbnails(ctx context.Context, ffmpegBin, input string, durationSec float64, dir string) (ThumbnailsMeta, error) {
	if durationSec <= 0 {
		return ThumbnailsMeta{}, fmt.Errorf("unknown duration")
	}
	frames := int(math.Min(thumbMaxFrames, math.Ceil(durationSec/thumbIntervalSec)))
	if frames < 1 {
		frames = 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ThumbnailsMeta{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	pattern := filepath.Join(dir, "frame_%03d.jpg")
	cmd := exec.CommandContext(ctx, ffmpegBin, "-v", "error", "-y",
		"-i", input,
		"-vf", fmt.Sprintf("fps=1/%d,scale=%d:%d", thumbIntervalSec, thumbCellWidth, thumbCellHeight),
		"-frames:v", strconv.Itoa(frames),
		pattern)
	if out, err := cmd.CombinedOutput(); err != nil {
		return ThumbnailsMeta{}, fmt.Errorf("extract thumbnails: %w: %s", err, strings.TrimSpace(string(out)))
	}

	count := countThumbnails(dir)
	if count == 0 {
		return ThumbnailsMeta{}, fmt.Errorf("no thumbnail frames produced")
	}
	return ThumbnailsMeta{
		Count:       count,
		IntervalSec: thumbIntervalSec,
		Width:       thumbCellWidth,
		Height:      thumbCellHeight,
	}, nil
}

func countThumbnails(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "frame_") && strings.HasSuffix(e.Name(), ".jpg") {
			count++
		}
	}
	return count
}

// ThumbnailPath returns the on-disk path for the n-th thumbnail frame
// (1-based), or an empty string when out of range.
func ThumbnailPath(dir string, n int) string {
	if n < 1 || n > countThumbnails(dir) {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("frame_%03d.jpg", n))
}
