package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// AudioStream describes one audio stream inside a video file.
type AudioStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
}

// ProbeAudioStreams lists the audio streams of a video via ffprobe.
func ProbeAudioStreams(ctx context.Context, ffprobeBin, input string) ([]AudioStream, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobeBin, "-v", "error", "-print_format", "json",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language,title",
		input)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe audio: %w", err)
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
		return nil, fmt.Errorf("parse ffprobe audio output: %w", err)
	}
	streams := []AudioStream{}
	for _, st := range raw.Streams {
		if st.CodecType != "audio" {
			continue
		}
		streams = append(streams, AudioStream{
			Index:    st.Index,
			Codec:    st.CodecName,
			Language: st.Tags.Language,
			Title:    st.Tags.Title,
		})
	}
	return streams, nil
}
