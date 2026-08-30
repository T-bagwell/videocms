package media

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	hlsSegmentDur   = 6
	hlsIdleTimeout  = 15 * time.Minute
	hlsManifestWait = 6 * time.Second
	// hlsMaxWidth caps the largest rendition; transcoding 4K at native
	// resolution would be too heavy for a self-hosted box.
	hlsMaxWidth = 1280
)

// rendition is one quality level in the adaptive HLS ladder.
type rendition struct {
	Name     string // e.g. "v1280"; used as subdirectory + playlist name
	Width    int
	Height   int
	BitrateK int
}

// HLSSubtitle describes one subtitle track advertised in the master playlist.
type HLSSubtitle struct {
	ID     string
	Name   string
	Active bool
}

// HLSAudio describes one audio track advertised in the master playlist as an
// HLS audio group, so players can switch audio without restarting.
type HLSAudio struct {
	Index int    // absolute stream index inside the source file
	Name  string // display name (language/title)
}

// renditionPlan builds a downward ladder from the source resolution, capped at
// hlsMaxWidth. Renditions wider than the source are skipped; for unknown source
// dimensions it falls back to a single 1280px rendition.
func renditionPlan(srcWidth, srcHeight int) []rendition {
	if srcWidth <= 0 || srcHeight <= 0 {
		// Probe failed or dimensions are unknown: fall back to a single
		// 1280px rendition so playback still works.
		return []rendition{{Name: "v1280", Width: hlsMaxWidth, Height: 720, BitrateK: 2500}}
	}
	targets := []struct{ width, bitrateK int }{
		{hlsMaxWidth, 2500},
		{854, 1500},
		{640, 900},
		{426, 500},
	}
	var out []rendition
	for _, t := range targets {
		if t.width > srcWidth {
			continue
		}
		height := 0
		height = int(math.Round(float64(t.width)*float64(srcHeight)/float64(srcWidth)/2)) * 2
		if height < 2 {
			height = 2
		}
		out = append(out, rendition{
			Name:     fmt.Sprintf("v%d", t.width),
			Width:    t.width,
			Height:   height,
			BitrateK: t.bitrateK,
		})
	}
	if len(out) == 0 {
		// Source is narrower than the smallest ladder step: keep one rendition
		// so the master playlist is never empty.
		height := int(math.Round(float64(426)*float64(srcHeight)/float64(srcWidth)/2)) * 2
		if height < 2 {
			height = 2
		}
		out = append(out, rendition{Name: "v426", Width: 426, Height: height, BitrateK: 500})
	}
	return out
}

// buildMaster returns the master playlist referencing every rendition plus
// the video's subtitle tracks (if any) as an HLS subtitle group and its audio
// tracks (if any) as an HLS audio group.
func buildMaster(rends []rendition, subs []HLSSubtitle, audios ...HLSAudio) []byte {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	for i, s := range subs {
		def := "NO"
		if s.Active || i == 0 {
			def = "YES"
		}
		name := strings.ReplaceAll(s.Name, `"`, "'")
		if name == "" {
			name = fmt.Sprintf("Subtitle %d", i+1)
		}
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=\"subs\",NAME=\"%s\",DEFAULT=%s,AUTOSELECT=YES,URI=\"subs/%s/playlist.m3u8\"\n",
			name, def, s.ID)
	}
	for i, au := range audios {
		def := "NO"
		if i == 0 {
			def = "YES"
		}
		name := strings.ReplaceAll(au.Name, `"`, "'")
		if name == "" {
			name = fmt.Sprintf("Audio %d", i+1)
		}
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",DEFAULT=%s,AUTOSELECT=YES,URI=\"a%d/index.m3u8\"\n",
			name, def, au.Index)
	}
	for _, r := range rends {
		audioAttr := ""
		if len(audios) > 0 {
			audioAttr = ",AUDIO=\"audio\""
		}
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d,RESOLUTION=%dx%d%s\n",
			r.BitrateK*1000, r.BitrateK*800, r.Width, r.Height, audioAttr)
		b.WriteString(r.Name + "/index.m3u8\n")
	}
	return []byte(b.String())
}

// hlsSubdirRe matches both video rendition directories (v1280) and audio-only
// track directories (a1) inside an HLS session.
var hlsSubdirRe = regexp.MustCompile(`^(v\d+|a\d+)$`)

type HLSSession struct {
	cancel     context.CancelFunc
	startSec   float64
	lastAccess time.Time
}

type HLSManager struct {
	dataDir  string
	ffmpeg   string
	mu       sync.Mutex
	sessions map[uuid.UUID]*HLSSession
}

func NewHLSManager(dataDir, ffmpegBin string) *HLSManager {
	m := &HLSManager{
		dataDir:  filepath.Join(dataDir, "hls"),
		ffmpeg:   ffmpegBin,
		sessions: make(map[uuid.UUID]*HLSSession),
	}
	_ = os.MkdirAll(m.dataDir, 0o755)
	go m.cleanupLoop()
	return m
}

func (m *HLSManager) sessionDir(videoID uuid.UUID) string {
	return filepath.Join(m.dataDir, videoID.String())
}

// Playlist ensures a transcode session exists for the video and returns the
// path of its master manifest. If startSec differs from the running session's
// offset by more than one segment, the session is restarted at that position.
func (m *HLSManager) Playlist(ctx context.Context, videoID uuid.UUID, input string, startSec float64, srcWidth, srcHeight int, subs []HLSSubtitle, audios ...HLSAudio) (string, error) {
	if startSec < 0 {
		startSec = 0
	}
	dir := m.sessionDir(videoID)

	m.mu.Lock()
	if sess, ok := m.sessions[videoID]; ok {
		if math.Abs(startSec-sess.startSec) <= hlsSegmentDur {
			sess.lastAccess = time.Now()
			m.mu.Unlock()
			return m.waitManifest(ctx, dir)
		}
		sess.cancel()
		delete(m.sessions, videoID)
		m.mu.Unlock()
		m.waitForExit(sess)
		_ = os.RemoveAll(dir)
	} else {
		m.mu.Unlock()
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	rends := renditionPlan(srcWidth, srcHeight)
	masterPath := filepath.Join(dir, "master.m3u8")
	if err := os.WriteFile(masterPath, buildMaster(rends, subs, audios...), 0o644); err != nil {
		return "", err
	}

	args := []string{"-v", "error", "-y", "-ss", fmt.Sprintf("%.2f", startSec), "-i", input}
	for _, r := range rends {
		outDir := filepath.Join(dir, r.Name)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", err
		}
		segPattern := filepath.Join(outDir, "seg_%05d.ts")
		manifest := filepath.Join(outDir, "index.m3u8")
		audioOpts := []string{"-map", "0:a:0?", "-c:a", "aac", "-b:a", "96k"}
		if len(audios) > 0 {
			// Audio is produced as separate HLS tracks below; video renditions
			// carry no audio to avoid duplicating it.
			audioOpts = []string{"-an"}
		}
		args = append(args,
			"-map", "0:v:0",
		)
		args = append(args, audioOpts...)
		args = append(args,
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-vf", "scale="+fmt.Sprint(r.Width)+":-2",
			"-force_key_frames", "expr:gte(t,n_forced*6)",
			"-f", "hls",
			"-hls_time", fmt.Sprint(hlsSegmentDur),
			"-hls_list_size", "0",
			"-hls_flags", "independent_segments",
			"-hls_segment_filename", segPattern,
			manifest,
		)
	}
	for _, au := range audios {
		outDir := filepath.Join(dir, fmt.Sprintf("a%d", au.Index))
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", err
		}
		segPattern := filepath.Join(outDir, "seg_%05d.ts")
		manifest := filepath.Join(outDir, "index.m3u8")
		args = append(args,
			"-map", fmt.Sprintf("0:%d", au.Index),
			"-c:a", "aac", "-b:a", "128k",
			"-f", "hls",
			"-hls_time", fmt.Sprint(hlsSegmentDur),
			"-hls_list_size", "0",
			"-hls_flags", "independent_segments",
			"-hls_segment_filename", segPattern,
			manifest,
		)
	}

	sctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(sctx, m.ffmpeg, args...)
	if stderr, err := cmd.StderrPipe(); err == nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					log.Printf("[hls:%s] %s", videoID.String()[:8], strings.TrimSpace(string(buf[:n])))
				}
				if err != nil {
					return
				}
			}
		}()
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("start ffmpeg hls: %w", err)
	}
	go func() {
		if err := cmd.Wait(); err != nil && sctx.Err() == nil {
			log.Printf("[hls:%s] ffmpeg exited: %v", videoID.String()[:8], err)
		}
		// manifests grow while transcoding; signal completion only when the
		// transcode actually finished (not when the session was cancelled)
		if sctx.Err() == nil {
			for _, r := range rends {
				if f, ferr := os.OpenFile(filepath.Join(dir, r.Name, "index.m3u8"), os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
					_, _ = f.WriteString("#EXT-X-ENDLIST\n")
					_ = f.Close()
				}
			}
			for _, au := range audios {
				if f, ferr := os.OpenFile(filepath.Join(dir, fmt.Sprintf("a%d", au.Index), "index.m3u8"), os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
					_, _ = f.WriteString("#EXT-X-ENDLIST\n")
					_ = f.Close()
				}
			}
		}
	}()

	m.mu.Lock()
	m.sessions[videoID] = &HLSSession{cancel: cancel, startSec: startSec, lastAccess: time.Now()}
	m.mu.Unlock()

	return m.waitManifest(ctx, dir)
}

// waitManifest waits until transcoding produced the first variant playlist.
// The master playlist is written up front, so its presence is not meaningful.
func (m *HLSManager) waitManifest(ctx context.Context, dir string) (string, error) {
	master := filepath.Join(dir, "master.m3u8")
	deadline := time.Now().Add(hlsManifestWait)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() || !hlsSubdirRe.MatchString(e.Name()) {
					continue
				}
				if st, err := os.Stat(filepath.Join(dir, e.Name(), "index.m3u8")); err == nil && st.Size() > 0 {
					return master, nil
				}
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("hls manifest did not appear within %s", hlsManifestWait)
}

func (m *HLSManager) waitForExit(sess *HLSSession) {
	// give the killed ffmpeg a moment to release files
	time.Sleep(300 * time.Millisecond)
}

// SessionFile returns the on-disk path for a variant playlist or segment under
// a session directory, validating both the name pattern and the path so
// requests can never escape the session.
func (m *HLSManager) SessionFile(videoID uuid.UUID, name string) (string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) != 2 || !hlsSubdirRe.MatchString(parts[0]) {
		return "", false
	}
	if parts[1] != "index.m3u8" && !SegmentNameMatch(parts[1]) {
		return "", false
	}
	dir := m.sessionDir(videoID)
	path := filepath.Join(dir, name)
	if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
		return "", false
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return "", false
	}
	m.mu.Lock()
	if sess, ok := m.sessions[videoID]; ok {
		sess.lastAccess = time.Now()
	}
	m.mu.Unlock()
	return path, true
}

func (m *HLSManager) Stop(videoID uuid.UUID) {
	m.mu.Lock()
	sess, ok := m.sessions[videoID]
	if ok {
		sess.cancel()
		delete(m.sessions, videoID)
	}
	m.mu.Unlock()
	if ok {
		_ = os.RemoveAll(m.sessionDir(videoID))
	}
}

func (m *HLSManager) cleanupLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var stale []uuid.UUID
		m.mu.Lock()
		for id, sess := range m.sessions {
			if now.Sub(sess.lastAccess) > hlsIdleTimeout {
				sess.cancel()
				delete(m.sessions, id)
				stale = append(stale, id)
			}
		}
		m.mu.Unlock()
		for _, id := range stale {
			_ = os.RemoveAll(m.sessionDir(id))
			log.Printf("[hls] reaped idle session %s", id.String()[:8])
		}
	}
}
