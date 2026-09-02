package media

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recorder claims due recordings and captures the channel stream with ffmpeg
// into DATA_DIR/recordings/<id>.ts.
type Recorder struct {
	pool    *pgxpool.Pool
	dataDir string
	ffmpeg  string
}

func NewRecorder(pool *pgxpool.Pool, dataDir, ffmpeg string) *Recorder {
	return &Recorder{pool: pool, dataDir: dataDir, ffmpeg: ffmpeg}
}

// Run polls for due recordings every 15 seconds until ctx is done.
func (r *Recorder) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	r.Process(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Process(ctx)
		}
	}
}

// Process starts every due recording and marks overdue pending schedules as
// failed. Recording itself runs in a goroutine per item.
func (r *Recorder) Process(ctx context.Context) {
	now := time.Now()
	rows, err := r.pool.Query(ctx, `
		SELECT rec.id, c.source_url, rec.title, rec.end_utc
		FROM recordings rec JOIN iptv_channels c ON c.id = rec.channel_id
		WHERE rec.status='pending' AND rec.start_utc <= $1 AND rec.end_utc > $1`,
		now)
	if err != nil {
		log.Printf("recorder: query due recordings: %v", err)
		return
	}
	type job struct {
		id    uuid.UUID
		url   string
		title string
		end   time.Time
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.id, &j.url, &j.title, &j.end); err == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	for _, j := range jobs {
		if j.url == "" {
			continue
		}
		tag, err := r.pool.Exec(ctx,
			`UPDATE recordings SET status='recording', updated_at=now()
			 WHERE id=$1 AND status='pending'`, j.id)
		if err != nil || tag.RowsAffected() == 0 {
			continue
		}
		go r.record(ctx, j.id, j.url, j.title, j.end)
	}
	if _, err := r.pool.Exec(ctx, `
		UPDATE recordings SET status='failed', error='recording window passed', updated_at=now()
		WHERE status='pending' AND end_utc <= $1`, now); err != nil {
		log.Printf("recorder: mark overdue: %v", err)
	}
}

func (r *Recorder) record(ctx context.Context, id uuid.UUID, sourceURL, title string, end time.Time) {
	dir := filepath.Join(r.dataDir, "recordings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.fail(id, err.Error())
		return
	}
	dest := filepath.Join(dir, id.String()+".ts")
	remaining := time.Until(end)
	if remaining < 1*time.Second {
		remaining = 1 * time.Second
	}
	cmd := exec.CommandContext(ctx, r.ffmpeg, "-y", "-i", sourceURL,
		"-t", fmt.Sprintf("%.0f", remaining.Seconds()),
		"-c", "copy", "-f", "mpegts", dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(dest)
		r.fail(id, strings.TrimSpace(string(out))[:min(len(out), 300)])
		return
	}
	if _, err := r.pool.Exec(context.Background(), `
		UPDATE recordings SET status='done', file_path=$1, updated_at=now() WHERE id=$2`,
		dest, id); err != nil {
		log.Printf("recorder: mark done: %v", err)
	}
}

func (r *Recorder) fail(id uuid.UUID, msg string) {
	if _, err := r.pool.Exec(context.Background(), `
		UPDATE recordings SET status='failed', error=$1, updated_at=now() WHERE id=$2`,
		msg, id); err != nil {
		log.Printf("recorder: mark failed: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
