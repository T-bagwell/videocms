package media

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Transcoder claims queued pre-transcode jobs (highest priority first) and
// warms the HLS session for the video. Local workers scale via
// TRANSCODE_WORKERS; the same queue protocol can later serve remote agents.
type Transcoder struct {
	pool *pgxpool.Pool
	hls  *HLSManager
}

func NewTranscoder(pool *pgxpool.Pool, hls *HLSManager) *Transcoder {
	return &Transcoder{pool: pool, hls: hls}
}

// Run polls for queued jobs every 5 seconds until ctx is done.
func (t *Transcoder) Run(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	t.Process(ctx, workers)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.Process(ctx, workers)
		}
	}
}

// Process claims up to workers queued jobs and transcodes them.
func (t *Transcoder) Process(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		var id, videoID uuid.UUID
		err := t.pool.QueryRow(ctx, `
			UPDATE transcode_jobs SET status='running', started_at=now()
			WHERE id = (
				SELECT id FROM transcode_jobs
				WHERE status='queued'
				ORDER BY priority DESC, created_at ASC
				LIMIT 1)
			RETURNING id, video_id`).Scan(&id, &videoID)
		if err != nil {
			return // no more queued jobs
		}
		t.runJob(ctx, id, videoID)
	}
}

func (t *Transcoder) runJob(ctx context.Context, id, videoID uuid.UUID) {
	var path string
	var width, height int
	var hdr bool
	if err := t.pool.QueryRow(ctx,
		`SELECT file_path, width, height, hdr FROM videos WHERE id=$1`, videoID).
		Scan(&path, &width, &height, &hdr); err != nil {
		t.fail(ctx, id, err.Error())
		return
	}
	_, err := t.hls.Playlist(ctx, videoID, path, 0, width, height, hdr, nil)
	if err != nil {
		t.fail(ctx, id, err.Error())
		return
	}
	if _, err := t.pool.Exec(ctx, `
		UPDATE transcode_jobs SET status='done', finished_at=now() WHERE id=$1`, id); err != nil {
		log.Printf("transcoder: mark done: %v", err)
	}
}

func (t *Transcoder) fail(ctx context.Context, id uuid.UUID, msg string) {
	if _, err := t.pool.Exec(ctx, `
		UPDATE transcode_jobs SET status='failed', error=$1, finished_at=now() WHERE id=$2`,
		msg, id); err != nil {
		log.Printf("transcoder: mark failed: %v", err)
	}
}
