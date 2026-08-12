package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// imageSubtitleCodecs cannot be converted to WebVTT without OCR; they are
// skipped by the embedded-subtitle extractor.
var imageSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
	"xsub":              true,
	"dvdsub":            true,
	"vobsub":            true,
}

// IsImageSubtitleCodec reports whether an ffmpeg subtitle codec is image-based
// and therefore cannot be extracted to WebVTT.
func IsImageSubtitleCodec(codec string) bool {
	return imageSubtitleCodecs[strings.ToLower(codec)]
}

type SubtitleStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
}

// ProbeSubtitleStreams lists the subtitle streams of a video. The video stream
// is ignored; only codec_type == "subtitle" streams are returned.
func ProbeSubtitleStreams(ctx context.Context, ffprobeBin, input string) ([]SubtitleStream, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobeBin, "-v", "error", "-print_format", "json",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language,title",
		input)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe subtitles: %w", err)
	}

	var raw struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			CodecType string `json:"codec_type"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe subtitle output: %w", err)
	}
	var streams []SubtitleStream
	for _, st := range raw.Streams {
		if st.CodecType != "subtitle" {
			continue
		}
		streams = append(streams, SubtitleStream{
			Index:    st.Index,
			Codec:    st.CodecName,
			Language: st.Tags.Language,
			Title:    st.Tags.Title,
		})
	}
	return streams, nil
}

// FirstTextSubtitle returns the first subtitle stream that can be converted to
// WebVTT, or -1 if there is none.
func FirstTextSubtitle(streams []SubtitleStream) int {
	for _, s := range streams {
		if !IsImageSubtitleCodec(s.Codec) {
			return s.Index
		}
	}
	return -1
}

// ExtractEmbeddedSubtitle extracts one subtitle stream from a video to a WebVTT
// file at dst. Image-based subtitles are rejected by the caller.
func ExtractEmbeddedSubtitle(ctx context.Context, ffmpegBin, input string, streamIndex int, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegBin, "-v", "error", "-y",
		"-i", input,
		"-map", fmt.Sprintf("0:s:%d", streamIndex),
		"-f", "webvtt", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract embedded subtitle: %w: %s", err, truncate(string(out), 300))
	}
	if st, err := os.Stat(dst); err != nil || st.Size() == 0 {
		return fmt.Errorf("extracted subtitle is empty")
	}
	return nil
}
