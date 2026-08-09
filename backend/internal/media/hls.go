package media

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	hlsSegmentDur   = 6
	hlsMaxWidth     = 1280
	hlsIdleTimeout  = 15 * time.Minute
	hlsManifestWait = 6 * time.Second
)

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
	os.MkdirAll(m.dataDir, 0o755)
	go m.cleanupLoop()
	return m
}

func (m *HLSManager) sessionDir(videoID uuid.UUID) string {
	return filepath.Join(m.dataDir, videoID.String())
}

// Playlist ensures a transcode session exists for the video and returns the
// path of its generated manifest. If startSec differs from the running
// session's offset by more than one segment, the session is restarted at that
// position.
func (m *HLSManager) Playlist(ctx context.Context, videoID uuid.UUID, input string, startSec float64) (string, error) {
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
		os.RemoveAll(dir)
	} else {
		m.mu.Unlock()
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	segPattern := filepath.Join(dir, "seg_%05d.ts")
	manifestPath := filepath.Join(dir, "index.m3u8")

	args := []string{
		"-v", "error", "-y",
		"-ss", fmt.Sprintf("%.2f", startSec),
		"-i", input,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-vf", "scale=" + fmt.Sprint(hlsMaxWidth) + ":-2",
		"-force_key_frames", "expr:gte(t,n_forced*6)",
		"-c:a", "aac", "-b:a", "128k",
		"-f", "hls",
		"-hls_time", fmt.Sprint(hlsSegmentDur),
		"-hls_list_size", "0",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", segPattern,
		manifestPath,
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
		// the manifest grows while transcoding; signal completion only when the
		// transcode actually finished (not when the session was cancelled)
		if sctx.Err() == nil {
			if f, ferr := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
				f.WriteString("#EXT-X-ENDLIST\n")
				f.Close()
			}
		}
	}()

	m.mu.Lock()
	m.sessions[videoID] = &HLSSession{cancel: cancel, startSec: startSec, lastAccess: time.Now()}
	m.mu.Unlock()

	return m.waitManifest(ctx, dir)
}

func (m *HLSManager) waitManifest(ctx context.Context, dir string) (string, error) {
	path := filepath.Join(dir, "index.m3u8")
	deadline := time.Now().Add(hlsManifestWait)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			return path, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("hls manifest did not appear within %s", hlsManifestWait)
}

func (m *HLSManager) waitForExit(sess *HLSSession) {
	// give the killed ffmpeg a moment to release files
	time.Sleep(300 * time.Millisecond)
}

func (m *HLSManager) Segment(videoID uuid.UUID, name string) (string, bool) {
	if !SegmentNameMatch(name) {
		return "", false
	}
	dir := m.sessionDir(videoID)
	path := filepath.Join(dir, name)
	cleanDir := filepath.Clean(dir)
	if !strings.HasPrefix(filepath.Clean(path), cleanDir+string(os.PathSeparator)) {
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
		os.RemoveAll(m.sessionDir(videoID))
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
			os.RemoveAll(m.sessionDir(id))
			log.Printf("[hls] reaped idle session %s", id.String()[:8])
		}
	}
}
