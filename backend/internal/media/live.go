package media

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// LiveManager converts an RTMP ingest stream into a rolling HLS playlist that
// the web player can watch, and tracks the running ffmpeg process per stream.
type LiveManager struct {
	dataDir string
	ffmpeg  string
	mu      sync.Mutex
	procs   map[uuid.UUID]*exec.Cmd
	cancel  map[uuid.UUID]context.CancelFunc
}

func NewLiveManager(dataDir, ffmpegBin string) *LiveManager {
	return &LiveManager{
		dataDir: filepath.Join(dataDir, "live"),
		ffmpeg:  ffmpegBin,
		procs:   make(map[uuid.UUID]*exec.Cmd),
		cancel:  make(map[uuid.UUID]context.CancelFunc),
	}
}

func (m *LiveManager) StreamDir(streamID uuid.UUID) string {
	return filepath.Join(m.dataDir, streamID.String())
}

// Start pulls rtmpURL and writes a rolling HLS playlist until the ingest
// stream stops or Stop is called.
func (m *LiveManager) Start(ctx context.Context, streamID uuid.UUID, rtmpURL string) error {
	outDir := m.StreamDir(streamID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	segPattern := filepath.Join(outDir, "seg_%05d.ts")
	manifest := filepath.Join(outDir, "index.m3u8")
	args := []string{
		"-v", "error", "-y", "-re",
		"-i", rtmpURL,
		"-c:v", "copy", "-c:a", "aac", "-b:a", "128k",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_filename", segPattern,
		manifest,
	}
	sctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(sctx, m.ffmpeg, args...)
	if stderr, err := cmd.StderrPipe(); err == nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					log.Printf("[live:%s] %s", streamID.String()[:8], strings.TrimSpace(string(buf[:n])))
				}
				if err != nil {
					return
				}
			}
		}()
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start live ffmpeg: %w", err)
	}
	m.mu.Lock()
	m.procs[streamID] = cmd
	m.cancel[streamID] = cancel
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		delete(m.procs, streamID)
		delete(m.cancel, streamID)
		m.mu.Unlock()
	}()
	return nil
}

func (m *LiveManager) Stop(streamID uuid.UUID) {
	m.mu.Lock()
	cancel, ok := m.cancel[streamID]
	cmd := m.procs[streamID]
	if ok {
		delete(m.cancel, streamID)
	}
	if cmd != nil {
		delete(m.procs, streamID)
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (m *LiveManager) StopAll() {
	m.mu.Lock()
	ids := make([]uuid.UUID, 0, len(m.procs))
	for id := range m.procs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}
