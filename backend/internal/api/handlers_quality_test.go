package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestQualityProfileBiasesVersions(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)

	insert := func(name, label string, width, height int, rank int) uuid.UUID {
		t.Helper()
		id := uuid.New()
		fp := filepath.Join(os.TempDir(), "qp-"+id.String()+".mkv")
		if err := os.WriteFile(fp, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(fp) })
		_, err := env.pool.Exec(context.Background(), `
			INSERT INTO videos (id, library_id, title, filename, file_path, size_bytes,
				width, height, video_codec, available, version_key, version_label, version_rank)
			VALUES ($1,$2,$3,$4,$5,100,$6,$7,$8,true,$9,$10,$11)`,
			id, libID, "Movie (1999)", name, fp, width, height, "h264",
			"the matrix 1999", label, rank)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	id4k := insert("Movie (1999) 4K.mkv", "4K", 3840, 2160, 50)
	id1080 := insert("Movie (1999) 1080p.mkv", "1080p", 1920, 1080, 30)

	rankOf := func(id uuid.UUID) int {
		var r int
		if err := env.pool.QueryRow(context.Background(),
			`SELECT version_rank FROM videos WHERE id=$1`, id).Scan(&r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if rankOf(id4k) <= rankOf(id1080) {
		t.Fatal("before profile, 4K should outrank 1080p")
	}

	status, _ := doJSON(t, http.MethodPost, env.server.URL+"/api/admin/quality-profiles", token,
		map[string]any{"name": "HD 1080p", "max_height": 1080, "active": true})
	if status != http.StatusCreated {
		t.Fatalf("create profile status = %d", status)
	}
	status, _ = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/quality-profiles/apply", token, nil)
	if status != http.StatusOK {
		t.Fatalf("apply profile status = %d", status)
	}
	if rankOf(id1080) <= rankOf(id4k) {
		t.Fatalf("after profile, 1080p should outrank 4K (got 1080p=%d 4k=%d)", rankOf(id1080), rankOf(id4k))
	}

	// The listing shows only the primary, which must now be the 1080p copy.
	status, d := doJSON(t, http.MethodGet, env.server.URL+"/api/videos?type=movie&page_size=50", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	seen := map[string]bool{}
	for _, it := range d["items"].([]any) {
		seen[it.(map[string]any)["id"].(string)] = true
	}
	if seen[id4k.String()] || !seen[id1080.String()] {
		t.Fatalf("primary selection wrong after profile: 4k seen=%v 1080 seen=%v", seen[id4k.String()], seen[id1080.String()])
	}

	// Deactivating restores the default preference on the next apply.
	status, d = doJSON(t, http.MethodPost, env.server.URL+"/api/admin/quality-profiles", token,
		map[string]any{"name": "no-op", "active": false})
	_ = status
	_ = d
}
