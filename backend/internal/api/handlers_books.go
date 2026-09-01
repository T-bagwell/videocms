package api

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
)

type book struct {
	ID          uuid.UUID `json:"id"`
	LibraryID   uuid.UUID `json:"library_id"`
	LibraryName string    `json:"library_name"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Format      string    `json:"format"`
	SizeBytes   int64     `json:"size_bytes"`
	Available   bool      `json:"available"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func visibleBooksCondition(userParam int) string {
	return fmt.Sprintf(`b.available AND NOT EXISTS (
		SELECT 1 FROM hidden_paths hp WHERE hp.user_id=$%d
		  AND (b.file_path = hp.path OR starts_with(b.file_path, hp.path || '/'))
	) AND NOT EXISTS (SELECT 1 FROM libraries lb WHERE lb.id = b.library_id AND lb.blocked)
	AND NOT EXISTS (SELECT 1 FROM blocked_titles bt
	                WHERE position(lower(bt.title) in lower(b.title)) > 0)`, userParam)
}

func scanBook(row pgx.Row) (book, error) {
	var b book
	err := row.Scan(&b.ID, &b.LibraryID, &b.LibraryName, &b.Title, &b.Author,
		&b.Format, &b.SizeBytes, &b.Available, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

// GET /api/books
func (a *App) listBooks(w http.ResponseWriter, r *http.Request) {
	args := []any{auth.UserFrom(r).ID}
	where := visibleBooksCondition(1)
	if libID := r.URL.Query().Get("library_id"); libID != "" {
		id, err := uuid.Parse(libID)
		if err == nil {
			where += fmt.Sprintf(` AND b.library_id=$%d`, len(args)+1)
			args = append(args, id)
		}
	}
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT b.id, b.library_id, l.name, b.title, b.author, b.format,
		       b.size_bytes, b.available, b.created_at, b.updated_at
		FROM books b JOIN libraries l ON l.id=b.library_id
		WHERE %s ORDER BY lower(b.title)`, where), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query books failed")
		return
	}
	defer rows.Close()
	items := []book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan book failed")
			return
		}
		items = append(items, b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/books/{id}
func (a *App) getBook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid book id")
		return
	}
	b, err := scanBook(a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT b.id, b.library_id, l.name, b.title, b.author, b.format,
		       b.size_bytes, b.available, b.created_at, b.updated_at
		FROM books b JOIN libraries l ON l.id=b.library_id
		WHERE b.id=$1 AND %s`, visibleBooksCondition(2)), id, auth.UserFrom(r).ID))
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query book failed")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// GET /api/books/{id}/file
func (a *App) bookFile(w http.ResponseWriter, r *http.Request) {
	b, fp, ok := a.bookFileFor(w, r)
	if !ok {
		return
	}
	ct := "application/octet-stream"
	if b.Format == "epub" {
		ct = "application/epub+zip"
	} else if b.Format == "cbz" {
		ct = "application/vnd.comicbook+zip"
	}
	w.Header().Set("Content-Type", ct)
	http.ServeFile(w, r, fp)
}

func (a *App) bookFileFor(w http.ResponseWriter, r *http.Request) (book, string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid book id")
		return book{}, "", false
	}
	var b book
	var filePath string
	err = a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT b.id, b.library_id, l.name, b.title, b.author, b.format,
		       b.size_bytes, b.available, b.created_at, b.updated_at
		FROM books b JOIN libraries l ON l.id=b.library_id
		WHERE b.id=$1 AND %s`, visibleBooksCondition(2)), id, auth.UserFrom(r).ID).
		Scan(&b.ID, &b.LibraryID, &b.LibraryName, &b.Title, &b.Author,
			&b.Format, &b.SizeBytes, &b.Available, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "book not found")
		return book{}, "", false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query book failed")
		return book{}, "", false
	}
	filePath, err = a.bookPath(b.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load book failed")
		return book{}, "", false
	}
	return b, filePath, true
}

func (a *App) bookPath(id uuid.UUID) (string, error) {
	var p string
	if err := a.pool.QueryRow(context.Background(),
		`SELECT file_path FROM books WHERE id=$1`, id).Scan(&p); err != nil {
		return "", err
	}
	return p, nil
}

var cbzImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".bmp": true,
}

// GET /api/books/{id}/pages (CBZ)
func (a *App) listCbzPages(w http.ResponseWriter, r *http.Request) {
	b, fp, ok := a.bookFileFor(w, r)
	if !ok {
		return
	}
	if b.Format != "cbz" {
		writeErr(w, http.StatusBadRequest, "not a comic archive")
		return
	}
	zr, err := zip.OpenReader(fp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open comic failed")
		return
	}
	defer func() { _ = zr.Close() }()
	var names []string
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && cbzImageExts[strings.ToLower(path.Ext(f.Name))] {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	pages := make([]map[string]any, 0, len(names))
	for i, n := range names {
		pages = append(pages, map[string]any{"n": i, "name": n})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages, "count": len(pages)})
}

// GET /api/books/{id}/pages/{n} (CBZ)
func (a *App) serveCbzPage(w http.ResponseWriter, r *http.Request) {
	b, fp, ok := a.bookFileFor(w, r)
	if !ok {
		return
	}
	if b.Format != "cbz" {
		writeErr(w, http.StatusBadRequest, "not a comic archive")
		return
	}
	n := 0
	if _, err := fmt.Sscanf(r.PathValue("n"), "%d", &n); err != nil || n < 0 {
		writeErr(w, http.StatusBadRequest, "invalid page number")
		return
	}
	zr, err := zip.OpenReader(fp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open comic failed")
		return
	}
	defer func() { _ = zr.Close() }()
	var names []string
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && cbzImageExts[strings.ToLower(path.Ext(f.Name))] {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	if n >= len(names) {
		writeErr(w, http.StatusNotFound, "page not found")
		return
	}
	var entry *zip.File
	for _, f := range zr.File {
		if f.Name == names[n] {
			entry = f
			break
		}
	}
	if entry == nil {
		writeErr(w, http.StatusNotFound, "page not found")
		return
	}
	rc, err := entry.Open()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open page failed")
		return
	}
	defer func() { _ = rc.Close() }()
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(entry.Name)))
	_, _ = io.Copy(w, rc)
}

type epubSpineEntry struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type epubMeta struct {
	Title    string           `json:"title"`
	Author   string           `json:"author"`
	Chapters []epubSpineEntry `json:"chapters"`
}

// GET /api/books/{id}/epub/spine
func (a *App) epubSpine(w http.ResponseWriter, r *http.Request) {
	b, fp, ok := a.bookFileFor(w, r)
	if !ok {
		return
	}
	if b.Format != "epub" {
		writeErr(w, http.StatusBadRequest, "not an epub")
		return
	}
	meta, err := parseEpubSpine(fp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "parse epub failed")
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// GET /api/books/{id}/epub/resource/{path...}
func (a *App) epubResource(w http.ResponseWriter, r *http.Request) {
	b, fp, ok := a.bookFileFor(w, r)
	if !ok {
		return
	}
	if b.Format != "epub" {
		writeErr(w, http.StatusBadRequest, "not an epub")
		return
	}
	name, err := url.PathUnescape(r.PathValue("path"))
	if err != nil || name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		writeErr(w, http.StatusBadRequest, "invalid resource path")
		return
	}
	name = strings.TrimPrefix(name, "/")
	zr, err := zip.OpenReader(fp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open epub failed")
		return
	}
	defer func() { _ = zr.Close() }()
	var entry *zip.File
	for _, f := range zr.File {
		if strings.TrimPrefix(f.Name, "/") == name {
			entry = f
			break
		}
	}
	if entry == nil {
		writeErr(w, http.StatusNotFound, "resource not found")
		return
	}
	rc, err := entry.Open()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "open resource failed")
		return
	}
	defer func() { _ = rc.Close() }()
	ct := mime.TypeByExtension(path.Ext(entry.Name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = io.Copy(w, rc)
}

func parseEpubSpine(filePath string) (epubMeta, error) {
	var meta epubMeta
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return meta, err
	}
	defer func() { _ = zr.Close() }()

	read := func(name string) ([]byte, error) {
		for _, f := range zr.File {
			if strings.TrimPrefix(f.Name, "/") == name {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer func() { _ = rc.Close() }()
				return io.ReadAll(io.LimitReader(rc, 8<<20))
			}
		}
		return nil, errors.New("missing " + name)
	}

	container, err := read("META-INF/container.xml")
	if err != nil {
		return meta, err
	}
	var c struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(container, &c); err != nil || len(c.Rootfiles) == 0 {
		return meta, errors.New("invalid container.xml")
	}
	opfPath := strings.TrimPrefix(c.Rootfiles[0].FullPath, "/")
	opf, err := read(opfPath)
	if err != nil {
		return meta, err
	}
	var pkg struct {
		Metadata struct {
			Titles   []string `xml:"title"`
			Creators []string `xml:"creator"`
		} `xml:"metadata"`
		Manifest []struct {
			ID   string `xml:"id,attr"`
			Href string `xml:"href,attr"`
		} `xml:"manifest>item"`
		Spine []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"spine>itemref"`
	}
	if err := xml.Unmarshal(opf, &pkg); err != nil {
		return meta, err
	}
	if len(pkg.Metadata.Titles) > 0 {
		meta.Title = strings.TrimSpace(pkg.Metadata.Titles[0])
	}
	if len(pkg.Metadata.Creators) > 0 {
		meta.Author = strings.TrimSpace(pkg.Metadata.Creators[0])
	}
	hrefByID := map[string]string{}
	for _, it := range pkg.Manifest {
		hrefByID[it.ID] = it.Href
	}
	baseDir := path.Dir(opfPath)
	for _, it := range pkg.Spine {
		href, ok := hrefByID[it.IDRef]
		if !ok {
			continue
		}
		clean := path.Clean(path.Join(baseDir, href))
		if strings.HasPrefix(clean, "../") {
			continue
		}
		meta.Chapters = append(meta.Chapters, epubSpineEntry{Path: clean})
	}
	return meta, nil
}
