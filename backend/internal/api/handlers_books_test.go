package api

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/models"
)

func writeZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	zw := zip.NewWriter(out)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func makeTestCBZ(t *testing.T, path string) {
	t.Helper()
	pngData := makeValidPNG(t)
	writeZip(t, path, map[string][]byte{
		"page 01.png": pngData,
		"page 02.png": pngData,
	})
}

func makeTestEPUB(t *testing.T, path string) {
	t.Helper()
	writeZip(t, path, map[string][]byte{
		"mimetype": []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`),
		"OEBPS/content.opf": []byte(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Test Novel</dc:title>
    <dc:creator>Test Author</dc:creator>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="style.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`),
		"OEBPS/ch1.xhtml": []byte(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 1</title></head>
<body><h1>Chapter 1</h1><p>Hello reader.</p></body></html>`),
		"OEBPS/style.css": []byte("body { color: black; }\n"),
	})
}

func TestBooksFromScan(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()
	libDir := t.TempDir()
	pngData := makeValidPNG(t)
	makeTestCBZ(t, filepath.Join(libDir, "Comic.cbz"))
	makeTestEPUB(t, filepath.Join(libDir, "Novel.epub"))

	var libID uuid.UUID
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, path) VALUES ($1, $2) RETURNING id`,
		"books", libDir).Scan(&libID); err != nil {
		t.Fatal(err)
	}
	if !env.app.scanner.Start(ctx, models.Library{ID: libID, Name: "books", Path: libDir}) {
		t.Fatal("scan did not start")
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		var finished *time.Time
		if err := env.pool.QueryRow(ctx,
			`SELECT scan_finished_at FROM libraries WHERE id=$1`, libID).Scan(&finished); err != nil {
			t.Fatal(err)
		}
		if finished != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	var n int
	if err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM books WHERE library_id=$1 AND available`, libID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("books scanned = %d, want 2", n)
	}

	token := loginAdmin(t, env)
	var books struct {
		Items []book `json:"items"`
	}
	resp := env.doJSON(t, http.MethodGet, "/api/books", nil, token, &books)
	if resp.StatusCode != http.StatusOK || len(books.Items) != 2 {
		t.Fatalf("books list status = %d items = %d", resp.StatusCode, len(books.Items))
	}
	var cbzID, epubID uuid.UUID
	for _, b := range books.Items {
		if b.Format == "cbz" {
			cbzID = b.ID
		}
		if b.Format == "epub" {
			epubID = b.ID
		}
	}
	if cbzID == uuid.Nil || epubID == uuid.Nil {
		t.Fatalf("missing formats: %+v", books.Items)
	}

	// CBZ pages.
	var pages struct {
		Pages []map[string]any `json:"pages"`
		Count int              `json:"count"`
	}
	resp = env.doJSON(t, http.MethodGet, "/api/books/"+cbzID.String()+"/pages", nil, token, &pages)
	if resp.StatusCode != http.StatusOK || pages.Count != 2 {
		t.Fatalf("cbz pages status = %d count = %d", resp.StatusCode, pages.Count)
	}
	req, _ := http.NewRequest(http.MethodGet,
		env.server.URL+"/api/books/"+cbzID.String()+"/pages/0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK || !bytes.Equal(body, pngData) {
		t.Fatalf("cbz page 0 status = %d, body len = %d", resp2.StatusCode, len(body))
	}

	// EPUB spine + resource.
	var spine struct {
		Title    string `json:"title"`
		Author   string `json:"author"`
		Chapters []struct {
			Path  string `json:"path"`
			Title string `json:"title"`
		} `json:"chapters"`
	}
	resp = env.doJSON(t, http.MethodGet, "/api/books/"+epubID.String()+"/epub/spine", nil, token, &spine)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("epub spine status = %d", resp.StatusCode)
	}
	if spine.Title != "Test Novel" || spine.Author != "Test Author" || len(spine.Chapters) != 1 {
		t.Fatalf("epub spine = %+v", spine)
	}
	if spine.Chapters[0].Path != "OEBPS/ch1.xhtml" {
		t.Errorf("spine path = %q", spine.Chapters[0].Path)
	}
	req, _ = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/books/"+epubID.String()+"/epub/resource/OEBPS/ch1.xhtml", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp2, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Hello reader")) {
		t.Fatalf("epub resource status = %d body = %.60s", resp2.StatusCode, body)
	}

	// Path traversal is rejected.
	req, _ = http.NewRequest(http.MethodGet,
		env.server.URL+"/api/books/"+epubID.String()+"/epub/resource/..%2F..%2Fetc%2Fpasswd", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp2, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("traversal status = %d, want 400", resp2.StatusCode)
	}
}
