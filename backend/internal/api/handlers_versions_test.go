package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/models"
)

// TestMultiVersionMovies exercises the full scan pipeline: real files named
// like multi-version releases are grouped, the best copy wins in listings,
// the detail endpoint lists every copy, and singletons never group.
func TestMultiVersionMovies(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()
	libDir := t.TempDir()
	src := makeTestVideo(t)

	files := []string{
		"The Matrix (1999) 1080p.mp4",
		"The Matrix (1999) 4K Extended Cut.mp4",
		"Inception (2010) 1080p.mp4",
		"Standalone.mp4",
	}
	for _, name := range files {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(libDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var libID uuid.UUID
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, path) VALUES ($1, $2) RETURNING id`,
		"versions", libDir).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	lib := models.Library{ID: libID, Name: "versions", Path: libDir}
	if !env.app.scanner.Start(ctx, lib) {
		t.Fatal("scan did not start")
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		var st string
		var finished *time.Time
		if err := env.pool.QueryRow(ctx,
			`SELECT scan_status, scan_finished_at FROM libraries WHERE id=$1`, libID).
			Scan(&st, &finished); err != nil {
			t.Fatal(err)
		}
		if finished != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var status string
	var finished *time.Time
	if err := env.pool.QueryRow(ctx,
		`SELECT scan_status, scan_finished_at FROM libraries WHERE id=$1`, libID).
		Scan(&status, &finished); err != nil {
		t.Fatal(err)
	}
	if finished == nil {
		t.Fatal("scan did not finish in time")
	}
	if status == "error" {
		var scanErr string
		_ = env.pool.QueryRow(ctx,
			`SELECT scan_error FROM libraries WHERE id=$1`, libID).Scan(&scanErr)
		t.Fatalf("scan failed: %s", scanErr)
	}

	idByFilename := map[string]uuid.UUID{}
	rows, err := env.pool.Query(ctx,
		`SELECT id, filename FROM videos WHERE library_id=$1`, libID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatal(err)
		}
		idByFilename[name] = id
	}
	rows.Close()
	for _, name := range files {
		if _, ok := idByFilename[name]; !ok {
			t.Fatalf("missing video %s in scan results", name)
		}
	}

	matrix1080 := idByFilename["The Matrix (1999) 1080p.mp4"]
	matrix4k := idByFilename["The Matrix (1999) 4K Extended Cut.mp4"]
	inception := idByFilename["Inception (2010) 1080p.mp4"]
	standalone := idByFilename["Standalone.mp4"]

	var key, label string
	var rank int
	if err := env.pool.QueryRow(ctx,
		`SELECT version_key, version_label, version_rank FROM videos WHERE id=$1`, matrix4k).
		Scan(&key, &label, &rank); err != nil {
		t.Fatal(err)
	}
	if key != "the matrix 1999" {
		t.Errorf("4K version key = %q, want %q", key, "the matrix 1999")
	}
	if label != "4K Extended Cut" {
		t.Errorf("4K version label = %q, want %q", label, "4K Extended Cut")
	}
	if rank <= 0 {
		t.Errorf("4K version rank = %d, want > 0", rank)
	}

	// Singletons must not keep a version key after the rebuild pass.
	for name, id := range map[string]uuid.UUID{
		"Inception (2010) 1080p.mp4": inception,
		"Standalone.mp4":             standalone,
	} {
		var k string
		if err := env.pool.QueryRow(ctx,
			`SELECT version_key FROM videos WHERE id=$1`, id).Scan(&k); err != nil {
			t.Fatal(err)
		}
		if k != "" {
			t.Errorf("%s: version_key = %q, want empty", name, k)
		}
	}

	token := env.loginAdmin(t)

	// Listings show only the best copy of a group.
	var list struct {
		Items []models.Video `json:"items"`
	}
	resp := env.doJSON(t, http.MethodGet, "/api/videos?type=movie&page_size=50", nil, token, &list)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	seen := map[uuid.UUID]bool{}
	for _, v := range list.Items {
		seen[v.ID] = true
	}
	if seen[matrix1080] {
		t.Error("non-primary 1080p version should be hidden from listings")
	}
	if !seen[matrix4k] {
		t.Error("primary 4K version should appear in listings")
	}
	if !seen[inception] || !seen[standalone] {
		t.Error("standalone movies should appear in listings")
	}

	// Detail page version list: all copies, best first, with primary marked.
	var versions struct {
		Versions  []models.Video `json:"versions"`
		PrimaryID *uuid.UUID     `json:"primary_id"`
	}
	resp = env.doJSON(t, http.MethodGet, "/api/videos/"+matrix1080.String()+"/versions", nil, token, &versions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("versions status = %d", resp.StatusCode)
	}
	if len(versions.Versions) != 2 {
		t.Fatalf("versions length = %d, want 2", len(versions.Versions))
	}
	if versions.Versions[0].ID != matrix4k {
		t.Errorf("first version = %s, want 4K %s", versions.Versions[0].ID, matrix4k)
	}
	if versions.PrimaryID == nil || *versions.PrimaryID != matrix4k {
		t.Errorf("primary_id = %v, want %s", versions.PrimaryID, matrix4k)
	}
	if !versions.Versions[0].IsPrimary || versions.Versions[1].IsPrimary {
		t.Error("only the 4K version should be marked primary")
	}
	if versions.Versions[0].VersionLabel != "4K Extended Cut" || versions.Versions[1].VersionLabel != "1080p" {
		t.Errorf("version labels = %q / %q", versions.Versions[0].VersionLabel, versions.Versions[1].VersionLabel)
	}

	// Direct access to a non-primary version still works.
	resp = env.doJSON(t, http.MethodGet, "/api/videos/"+matrix1080.String(), nil, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("direct access to non-primary version status = %d, want 200", resp.StatusCode)
	}
}
