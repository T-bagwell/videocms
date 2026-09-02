package api

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestTranscodeQueue(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertRealVideo(t, libID, "Transcode Me", makeTestVideo(t))

	status, d := doJSON(t, http.MethodPost, env.server.URL+"/api/admin/transcode/queue", token,
		map[string]any{"video_id": videoID.String(), "priority": 9})
	if status != http.StatusCreated {
		t.Fatalf("queue status = %d body = %v", status, d)
	}
	jobID := d["id"].(string)

	env.app.transcoder.Process(context.Background(), 1)

	var jobStatus, jobErr string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := env.pool.QueryRow(context.Background(),
			`SELECT status, error FROM transcode_jobs WHERE id=$1`, jobID).
			Scan(&jobStatus, &jobErr); err != nil {
			t.Fatal(err)
		}
		if jobStatus == "done" || jobStatus == "failed" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if jobStatus != "done" {
		t.Fatalf("job status = %s error = %q", jobStatus, jobErr)
	}

	status, d = doJSON(t, http.MethodGet, env.server.URL+"/api/admin/transcode/jobs", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("jobs status = %d body = %v", status, d)
	}
	if d["workers"].(float64) != 1 {
		t.Fatalf("workers = %v", d["workers"])
	}

	// Cancelling a queued job marks it failed.
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/transcode/queue", token,
		map[string]any{"video_id": videoID.String(), "priority": 1})
	if status != http.StatusCreated {
		t.Fatal("queue second job failed")
	}
	id2 := d["id"].(string)
	status, _ = doJSON(t, http.MethodDelete,
		env.server.URL+"/api/admin/transcode/jobs/"+id2, token, nil)
	if status != http.StatusOK {
		t.Fatalf("cancel status = %d", status)
	}
	var st string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT status FROM transcode_jobs WHERE id=$1`, id2).Scan(&st); err != nil {
		t.Fatal(err)
	}
	if st != "failed" {
		t.Fatalf("cancelled status = %s", st)
	}
}
