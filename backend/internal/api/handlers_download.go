package api

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

type audioTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

type subtitleTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

type sidecarTrack struct {
	ID    string `json:"id"`
	Lang  string `json:"lang"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// GET /api/videos/{id}/tracks — lists the streams available for a remuxed
// download: container, audio streams, embedded subtitle streams and sidecar
// subtitle tracks.
func (a *App) videoTracks(w http.ResponseWriter, r *http.Request) {
	path, ok := a.videoFileFor(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	ffprobe := media.ResolveTool("ffprobe")

	audioStreams, err := media.ProbeAudioStreams(r.Context(), ffprobe, path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "probe audio tracks failed")
		return
	}
	subStreams, err := media.ProbeSubtitleStreams(r.Context(), ffprobe, path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "probe subtitle tracks failed")
		return
	}
	videoID, _ := uuid.Parse(r.PathValue("id"))
	sidecars, err := a.listTracksForVideo(r.Context(), videoID, "", nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load subtitle tracks failed")
		return
	}

	audio := make([]audioTrack, 0, len(audioStreams))
	for _, s := range audioStreams {
		audio = append(audio, audioTrack{Index: s.Index, Codec: s.Codec, Language: s.Language, Title: s.Title})
	}
	subs := make([]subtitleTrack, 0, len(subStreams))
	for _, s := range subStreams {
		subs = append(subs, subtitleTrack{Index: s.Index, Codec: s.Codec, Language: s.Language, Title: s.Title})
	}
	sidecarOut := make([]sidecarTrack, 0, len(sidecars))
	for _, t := range sidecars {
		if t.Kind == "embedded" {
			continue // embedded streams are already listed above
		}
		sidecarOut = append(sidecarOut, sidecarTrack{ID: t.ID.String(), Lang: t.Lang, Title: t.Title, Kind: t.Kind})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"container": strings.TrimPrefix(filepath.Ext(path), "."),
		"audio":     audio,
		"subtitle":  subs,
		"sidecar":   sidecarOut,
	})
}

// GET /api/videos/{id}/download/remux?container=mp4|mkv&audio=<idx>&sub=<idx,...>&sidecar=<id,...>
// Streams a remuxed copy of the video with the selected audio and subtitle
// tracks. No re-encoding happens: streams are copied into the target container.
func (a *App) remuxDownload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid video id")
		return
	}
	var allowDownloads bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT allow_downloads FROM videos WHERE id=$1`, id).Scan(&allowDownloads); err != nil || !allowDownloads {
		writeErr(w, http.StatusForbidden, "downloads are disabled for this video")
		return
	}
	path, ok := a.videoFileFor(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found or unavailable")
		return
	}
	container := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("container")))
	if container == "" {
		container = "mkv"
	}
	if container != "mkv" && container != "mp4" {
		writeErr(w, http.StatusBadRequest, "container must be mkv or mp4")
		return
	}

	ffprobe := media.ResolveTool("ffprobe")
	audioStreams, err := media.ProbeAudioStreams(r.Context(), ffprobe, path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "probe audio tracks failed")
		return
	}
	subStreams, err := media.ProbeSubtitleStreams(r.Context(), ffprobe, path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "probe subtitle tracks failed")
		return
	}

	audioIdx := -1
	if q := r.URL.Query().Get("audio"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || !containsStreamIndex(audioStreams, n) {
			writeErr(w, http.StatusBadRequest, "audio stream not found")
			return
		}
		audioIdx = n
	} else if len(audioStreams) > 0 {
		audioIdx = audioStreams[0].Index
	}

	subSel := []int{}
	if q := r.URL.Query().Get("sub"); q != "" {
		for _, part := range strings.Split(q, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || !containsSubtitleIndex(subStreams, n) {
				writeErr(w, http.StatusBadRequest, "subtitle stream not found")
				return
			}
			if container == "mp4" && media.IsImageSubtitleCodec(codecForSubtitle(subStreams, n)) {
				writeErr(w, http.StatusBadRequest, "image-based subtitles cannot be muxed into MP4; use MKV instead")
				return
			}
			subSel = append(subSel, n)
		}
	}

	videoID, _ := uuid.Parse(r.PathValue("id"))
	sidecarIDs := []string{}
	if q := r.URL.Query().Get("sidecar"); q != "" {
		for _, part := range strings.Split(q, ",") {
			if id := strings.TrimSpace(part); id != "" {
				sidecarIDs = append(sidecarIDs, id)
			}
		}
	}
	sidecarPaths := []string{}
	for _, sid := range sidecarIDs {
		var trackPath string
		err := a.pool.QueryRow(r.Context(),
			`SELECT path FROM subtitle_tracks WHERE id=$1 AND video_id=$2 AND kind <> 'embedded'`,
			sid, videoID).Scan(&trackPath)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "sidecar subtitle not found")
			return
		}
		if _, err := os.Stat(trackPath); err != nil {
			writeErr(w, http.StatusBadRequest, "sidecar subtitle file is missing")
			return
		}
		sidecarPaths = append(sidecarPaths, trackPath)
	}

	args := []string{"-v", "error"}
	args = append(args, "-i", path)
	for _, sp := range sidecarPaths {
		args = append(args, "-i", sp)
	}
	args = append(args, "-map", "0:v:0?")
	if audioIdx >= 0 {
		args = append(args, "-map", fmt.Sprintf("0:%d", audioIdx))
	}
	for _, idx := range subSel {
		args = append(args, "-map", fmt.Sprintf("0:%d", idx))
	}
	for i := range sidecarPaths {
		args = append(args, "-map", fmt.Sprintf("%d:0", i+1))
	}
	args = append(args, "-c:v", "copy", "-c:a", "copy")
	if len(subSel) > 0 || len(sidecarPaths) > 0 {
		if container == "mp4" {
			args = append(args, "-c:s", "mov_text")
		} else {
			args = append(args, "-c:s", "copy")
		}
	}
	if container == "mp4" {
		args = append(args, "-movflags", "frag_keyframe+empty_moov")
	}
	args = append(args, "-f", containerFormat(container), "pipe:1")

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + "." + container
	w.Header().Set("Content-Type", containerContentType(container))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", mime.QEncoding.Encode("UTF-8", name)))
	w.Header().Set("Cache-Control", "no-store")

	cmd := exec.CommandContext(r.Context(), media.ResolveTool("ffmpeg"), args...)
	cmd.Stdout = w
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Headers are already sent at this point; only the log tells us why.
		log.Printf("remux download failed for %s: %v: %s", name, err, stderr.String())
	}
}

func containsStreamIndex(streams []media.AudioStream, index int) bool {
	for _, s := range streams {
		if s.Index == index {
			return true
		}
	}
	return false
}

func containsSubtitleIndex(streams []media.SubtitleStream, index int) bool {
	for _, s := range streams {
		if s.Index == index {
			return true
		}
	}
	return false
}

func codecForSubtitle(streams []media.SubtitleStream, index int) string {
	for _, s := range streams {
		if s.Index == index {
			return s.Codec
		}
	}
	return ""
}

func containerFormat(container string) string {
	if container == "mp4" {
		return "mp4"
	}
	return "matroska"
}

func containerContentType(container string) string {
	if container == "mp4" {
		return "video/mp4"
	}
	return "video/x-matroska"
}
