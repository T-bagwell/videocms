package media

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DownloadJob is one row from the downloads table that the worker is about to
// run.
type DownloadJob struct {
	ID           uuid.UUID
	URL          string
	TargetPath   string
	Format       string
	IntervalSecs int64
}

// Downloader runs yt-dlp jobs from the downloads table one at a time and keeps
// track of running processes so a job can be cancelled.
type Downloader struct {
	pool  *pgxpool.Pool
	bin   string
	mu    sync.Mutex
	procs map[uuid.UUID]*exec.Cmd
}

func NewDownloader(pool *pgxpool.Pool, bin string) *Downloader {
	return &Downloader{pool: pool, bin: bin, procs: map[uuid.UUID]*exec.Cmd{}}
}

// SetBin overrides the yt-dlp binary path (used by tests and tool resolution).
func (d *Downloader) SetBin(path string) {
	d.bin = path
}

// ResolveBin returns the configured yt-dlp binary, falling back to PATH.
func (d *Downloader) ResolveBin() string {
	if d.bin != "" {
		return d.bin
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	return "yt-dlp"
}

// Run polls for due jobs until ctx is cancelled.
func (d *Downloader) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			d.killAll()
			return
		case <-ticker.C:
			d.ProcessNext(ctx)
		}
	}
}

// ProcessNext claims and runs one due download job, if any. Exported for tests
// and potential manual triggering.
func (d *Downloader) ProcessNext(ctx context.Context) {
	var job DownloadJob
	var lastRun *time.Time
	err := d.pool.QueryRow(ctx, `
		SELECT id, url, target_path, format, interval_secs, last_run_at
		FROM downloads
		WHERE status = 'queued'
		   OR (status = 'completed' AND interval_secs > 0
		       AND (last_run_at IS NULL OR last_run_at <= now() - (interval_secs * interval '1 second')))
		ORDER BY created_at
		LIMIT 1`,
	).Scan(&job.ID, &job.URL, &job.TargetPath, &job.Format, &job.IntervalSecs, &lastRun)
	if err != nil {
		return // no due job (or query error; keep polling)
	}

	tag, err := d.pool.Exec(ctx,
		`UPDATE downloads SET status='downloading', error='', updated_at=now()
		 WHERE id=$1 AND status='queued'`, job.ID)
	if err != nil || tag.RowsAffected() != 1 {
		return // claimed by someone else
	}
	d.runJob(ctx, job)
}

func (d *Downloader) runJob(ctx context.Context, job DownloadJob) {
	bin := d.ResolveBin()
	args := []string{
		"--newline",
		"-f", job.Format,
		"-o", filepath.Join(job.TargetPath, "%(title)s.%(ext)s"),
		job.URL,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	d.mu.Lock()
	d.procs[job.ID] = cmd
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.procs, job.ID)
		d.mu.Unlock()
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.fail(job, "start yt-dlp: "+err.Error())
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		d.fail(job, "start yt-dlp: "+err.Error())
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if pct, ok := parseProgress(scanner.Text()); ok {
				d.updateProgress(job.ID, pct)
			}
		}
	}()
	runErr := cmd.Wait()
	<-done

	if runErr != nil {
		var st string
		_ = d.pool.QueryRow(context.Background(),
			`SELECT status FROM downloads WHERE id=$1`, job.ID).Scan(&st)
		if st == "canceled" {
			return // the job was cancelled while the process was dying
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		d.fail(job, msg)
		return
	}
	d.complete(job)
}

func (d *Downloader) complete(job DownloadJob) {
	_, _ = d.pool.Exec(context.Background(), `
		UPDATE downloads SET status='completed', progress=100, error='',
		       last_run_at=now(), updated_at=now()
		WHERE id=$1`, job.ID)
}

func (d *Downloader) fail(job DownloadJob, msg string) {
	_, _ = d.pool.Exec(context.Background(), `
		UPDATE downloads SET status='failed', error=$2, updated_at=now()
		WHERE id=$1 AND status <> 'canceled'`, job.ID, msg)
}

func (d *Downloader) updateProgress(id uuid.UUID, pct float64) {
	_, _ = d.pool.Exec(context.Background(), `
		UPDATE downloads SET progress=$2, updated_at=now()
		WHERE id=$1 AND status='downloading'`, id, pct)
}

// Cancel kills the running yt-dlp process for a job, if any.
func (d *Downloader) Cancel(id uuid.UUID) {
	d.mu.Lock()
	cmd := d.procs[id]
	d.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (d *Downloader) killAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, cmd := range d.procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// parseProgress extracts the percent value from a yt-dlp line such as
// "[download]  45.2% of 100.00MiB at 2.3MiB/s ETA 00:24".
func parseProgress(line string) (float64, bool) {
	i := strings.Index(line, "%")
	if i < 0 {
		return 0, false
	}
	s := line[:i]
	if j := strings.LastIndexAny(s, " \t"); j >= 0 {
		s = s[j+1:]
	}
	p, err := strconv.ParseFloat(s, 64)
	if err != nil || p < 0 || p > 100 {
		return 0, false
	}
	return p, true
}
