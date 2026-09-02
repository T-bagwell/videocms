-- Distributed transcoding queue: pre-transcode jobs with priorities claimed
-- by local workers (TRANSCODE_WORKERS) or, in future, remote agents.
CREATE TABLE IF NOT EXISTS transcode_jobs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    video_id    uuid NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    priority    int NOT NULL DEFAULT 5,
    status      text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'done', 'failed')),
    worker_id   text NOT NULL DEFAULT '',
    error       text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    started_at  timestamptz,
    finished_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_transcode_jobs_queue ON transcode_jobs(status, priority DESC, created_at);
