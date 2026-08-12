package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"videocms/backend/internal/config"
	"videocms/backend/internal/db"
)

// integrationEnv spins up a throwaway PostgreSQL database, applies migrations,
// seeds the admin user and starts the real HTTP stack. Tests skip when no
// reachable PostgreSQL is available (e.g. CI without a server).
type integrationEnv struct {
	t      *testing.T
	pool   *pgxpool.Pool
	server *httptest.Server
	dbName string
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()
	adminDSN := testAdminDSN(t)
	if adminDSN == "" {
		t.Skip("no reachable PostgreSQL for integration tests (set TEST_PG_DSN)")
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL: %v", err)
	}
	defer admin.Close()

	name := fmt.Sprintf("videocms_test_%d_%d", time.Now().UnixNano(), rand.Intn(10000))
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		t.Skipf("cannot create test database: %v", err)
	}
	env := &integrationEnv{t: t, dbName: name}
	t.Cleanup(func() {
		env.pool.Close()
		env.server.Close()
		ctx2 := context.Background()
		a, err := pgxpool.New(ctx2, adminDSN)
		if err == nil {
			defer a.Close()
			a.Exec(ctx2, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`)
		}
	})

	dsn := strings.Replace(adminDSN, "/postgres", "/"+name, 1)
	env.pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := db.Migrate(ctx, env.pool); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if err := db.SeedAdmin(ctx, env.pool, "admin", "admin123"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	cfg := config.Config{
		Addr:      ":0",
		DataDir:   t.TempDir(),
		JWTSecret: "integration-test-secret",
	}
	app, err := New(cfg, env.pool)
	if err != nil {
		t.Fatalf("build app: %v", err)
	}
	env.server = httptest.NewServer(app.Routes())
	return env
}

// testAdminDSN finds a working superuser DSN to create/drop the test database.
func testAdminDSN(t *testing.T) string {
	socketDSN := ""
	if os.Getenv("USER") != "" {
		socketDSN = "postgres://" + os.Getenv("USER") + "@/postgres?host=/var/run/postgresql&sslmode=disable"
	}
	candidates := []string{
		os.Getenv("TEST_PG_DSN"),
		"postgres://localhost:5432/postgres?sslmode=disable",
		socketDSN,
		"postgres:///postgres?host=/var/run/postgresql&sslmode=disable",
		"postgres:///postgres?sslmode=disable",
	}
	for _, dsn := range candidates {
		if dsn == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err := pool.Ping(ctx); err == nil {
				pool.Close()
				cancel()
				return dsn
			}
			pool.Close()
		}
		cancel()
	}
	return ""
}

func (e *integrationEnv) loginAdmin(t *testing.T) string {
	t.Helper()
	var resp struct {
		Token string `json:"token"`
	}
	e.doJSON(t, http.MethodPost, "/api/auth/login", map[string]any{
		"username": "admin",
		"password": "admin123",
	}, "", &resp)
	if resp.Token == "" {
		t.Fatal("admin login returned no token")
	}
	return resp.Token
}

func (e *integrationEnv) doJSON(t *testing.T, method, path string, body any, token string, out any) *http.Response {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.server.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("decode %s %s response: %v (%s)", method, path, err, data)
		}
	}
	return resp
}

func (e *integrationEnv) insertLibrary(t *testing.T, blocked bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	name := "lib-" + uuid.NewString()[:8]
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO libraries (name, path, blocked) VALUES ($1, $2, $3) RETURNING id`,
		name, "/tmp/"+name, blocked).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *integrationEnv) insertVideo(t *testing.T, libID uuid.UUID, title, ext string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	fp := "/tmp/videocms-it-" + uuid.NewString() + ext
	if err := os.WriteFile(fp, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(fp) })
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO videos (library_id, title, filename, file_path, size_bytes, duration_sec, width, height, available)
		VALUES ($1,$2,$3,$4,1234,60,1280,720,true) RETURNING id`,
		libID, title, filepath.Base(fp), fp).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *integrationEnv) insertSeriesVideo(t *testing.T, libID uuid.UUID, seriesName string, season, episode int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	videoID := e.insertVideo(t, libID, fmt.Sprintf("%s S%02dE%02d", seriesName, season, episode), ".mkv")
	var seriesID uuid.UUID
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO series (library_id, name, season, episode_count)
		VALUES ($1,$2,$3,1)
		ON CONFLICT (library_id, name, season) DO UPDATE SET episode_count = series.episode_count + 0
		RETURNING id`, libID, seriesName, season).Scan(&seriesID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(context.Background(),
		`UPDATE videos SET series_id=$1, season=$2, episode=$3 WHERE id=$4`,
		seriesID, season, episode, videoID); err != nil {
		t.Fatal(err)
	}
	return seriesID, videoID
}

func TestShareRequiresAuth(t *testing.T) {
	e := newIntegrationEnv(t)
	libID := e.insertLibrary(t, false)
	videoID := e.insertVideo(t, libID, "Movie A", ".mp4")
	resp := e.doJSON(t, http.MethodPost, "/api/videos/"+videoID.String()+"/share", map[string]any{"hours": 24}, "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("share without auth: got %d, want 401", resp.StatusCode)
	}
}

func TestShareLifecycle(t *testing.T) {
	e := newIntegrationEnv(t)
	token := e.loginAdmin(t)
	libID := e.insertLibrary(t, false)
	videoID := e.insertVideo(t, libID, "Movie B", ".mp4")

	var created struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	resp := e.doJSON(t, http.MethodPost, "/api/videos/"+videoID.String()+"/share",
		map[string]any{"hours": 24}, token, &created)
	if resp.StatusCode != http.StatusCreated || created.Token == "" {
		t.Fatalf("create share: status %d token %q", resp.StatusCode, created.Token)
	}

	resp = e.doJSON(t, http.MethodGet, "/api/share/"+created.Token+"/info", nil, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public info: got %d, want 200", resp.StatusCode)
	}
	resp = e.doJSON(t, http.MethodGet, "/api"+created.URL+"/video/"+videoID.String()+"/stream", nil, "", nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("public stream: got %d, want 200/206", resp.StatusCode)
	}

	resp = e.doJSON(t, http.MethodDelete, "/api/share/"+created.Token, nil, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke: got %d, want 200", resp.StatusCode)
	}
	resp = e.doJSON(t, http.MethodGet, "/api/share/"+created.Token+"/info", nil, "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("info after revoke: got %d, want 404", resp.StatusCode)
	}
}

func TestShareExpires(t *testing.T) {
	e := newIntegrationEnv(t)
	libID := e.insertLibrary(t, false)
	videoID := e.insertVideo(t, libID, "Movie C", ".mp4")
	expiredToken := uuid.NewString()
	if _, err := e.pool.Exec(context.Background(), `
		INSERT INTO share_tokens (scope, video_id, token, expires_at, created_by)
		VALUES ('video', $1, $2, now() - interval '1 hour', NULL)`,
		videoID, expiredToken); err != nil {
		t.Fatal(err)
	}
	resp := e.doJSON(t, http.MethodGet, "/api/share/"+expiredToken+"/info", nil, "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expired share info: got %d, want 404", resp.StatusCode)
	}
}

func TestShareBlockedLibrary(t *testing.T) {
	e := newIntegrationEnv(t)
	token := e.loginAdmin(t)
	libID := e.insertLibrary(t, true) // blocked
	videoID := e.insertVideo(t, libID, "Movie D", ".mp4")
	resp := e.doJSON(t, http.MethodPost, "/api/videos/"+videoID.String()+"/share",
		map[string]any{"hours": 24}, token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("share for blocked library: got %d, want 404", resp.StatusCode)
	}
}

func TestSeriesShare(t *testing.T) {
	e := newIntegrationEnv(t)
	token := e.loginAdmin(t)
	libID := e.insertLibrary(t, false)
	seriesID, ep1 := e.insertSeriesVideo(t, libID, "Test Show", 1, 1)
	_, ep2 := e.insertSeriesVideo(t, libID, "Test Show", 1, 2)

	var created struct {
		Token string `json:"token"`
	}
	resp := e.doJSON(t, http.MethodPost, "/api/series/"+seriesID.String()+"/share",
		map[string]any{"hours": 24}, token, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create series share: got %d, want 201", resp.StatusCode)
	}

	var info struct {
		Scope string `json:"scope"`
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
	}
	resp = e.doJSON(t, http.MethodGet, "/api/share/"+created.Token+"/info", nil, "", &info)
	if resp.StatusCode != http.StatusOK || info.Scope != "series" || len(info.Items) != 2 {
		t.Fatalf("series share info: status %d scope %q items %d", resp.StatusCode, info.Scope, len(info.Items))
	}
	resp = e.doJSON(t, http.MethodGet,
		"/api/share/"+created.Token+"/video/"+ep1.String()+"/stream", nil, "", nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("series episode stream: got %d, want 200/206", resp.StatusCode)
	}

	// a video outside the series must not be reachable through this token
	other := e.insertVideo(t, libID, "Unrelated", ".mp4")
	resp = e.doJSON(t, http.MethodGet,
		"/api/share/"+created.Token+"/video/"+other.String()+"/stream", nil, "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unrelated video via series token: got %d, want 404", resp.StatusCode)
	}
	_ = ep2
}

func TestSubtitleTrackSwitching(t *testing.T) {
	e := newIntegrationEnv(t)
	token := e.loginAdmin(t)
	libID := e.insertLibrary(t, false)
	videoID := e.insertVideo(t, libID, "Movie E", ".mp4")

	dir := t.TempDir()
	en := filepath.Join(dir, "en.srt")
	fr := filepath.Join(dir, "fr.srt")
	os.WriteFile(en, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0o644)
	os.WriteFile(fr, []byte("1\n00:00:01,000 --> 00:00:02,000\nBonjour\n"), 0o644)

	for i, p := range []string{en, fr} {
		lang := "en"
		if i == 1 {
			lang = "fr"
		}
		if _, err := e.pool.Exec(context.Background(), `
			INSERT INTO subtitle_tracks (video_id, position, lang, title, path, kind, source_key)
			VALUES ($1,$2,$3,$4,$5,'sidecar',$5)`,
			videoID, i, lang, lang, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.pool.Exec(context.Background(),
		`UPDATE videos SET subtitle_path=$1 WHERE id=$2`, en, videoID); err != nil {
		t.Fatal(err)
	}

	var list struct {
		Items []struct {
			ID       uuid.UUID `json:"id"`
			Lang     string    `json:"lang"`
			IsActive bool      `json:"is_active"`
		} `json:"items"`
	}
	resp := e.doJSON(t, http.MethodGet, "/api/videos/"+videoID.String()+"/subtitle-tracks", nil, token, &list)
	if resp.StatusCode != http.StatusOK || len(list.Items) != 2 {
		t.Fatalf("subtitle tracks: status %d items %d", resp.StatusCode, len(list.Items))
	}
	if !list.Items[0].IsActive || list.Items[1].IsActive {
		t.Fatalf("unexpected active flags: %+v", list.Items)
	}

	// switch to the French track and re-check
	resp = e.doJSON(t, http.MethodPut,
		"/api/videos/"+videoID.String()+"/subtitles/"+list.Items[1].ID.String()+"/active",
		nil, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate track: got %d, want 200", resp.StatusCode)
	}
	resp = e.doJSON(t, http.MethodGet, "/api/videos/"+videoID.String()+"/subtitle-tracks", nil, token, &list)
	if !list.Items[1].IsActive || list.Items[0].IsActive {
		t.Fatalf("active flag did not switch: %+v", list.Items)
	}

	// the track endpoint converts SRT -> WebVTT
	resp = e.doJSON(t, http.MethodGet,
		"/api/videos/"+videoID.String()+"/subtitles/"+list.Items[1].ID.String(), nil, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("serve track: got %d, want 200", resp.StatusCode)
	}
}

func TestExportEndpoints(t *testing.T) {
	e := newIntegrationEnv(t)
	token := e.loginAdmin(t)
	libID := e.insertLibrary(t, false)
	e.insertVideo(t, libID, "Movie F", ".mp4")

	req, _ := http.NewRequest(http.MethodGet, e.server.URL+"/api/admin/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin export: got %d", resp.StatusCode)
	}
	var dump map[string]any
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("admin export not JSON: %v", err)
	}
	if _, ok := dump["videos"]; !ok {
		t.Fatal("admin export missing videos section")
	}

	req, _ = http.NewRequest(http.MethodGet, e.server.URL+"/api/users/me/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user export: got %d", resp.StatusCode)
	}
}
