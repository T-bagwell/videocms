package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultChunkSize = 8 << 20 // 8 MiB
	maxChunkSize     = 16 << 20
)

type uploadSession struct {
	ID          uuid.UUID `json:"id"`
	Filename    string    `json:"filename"`
	TargetPath  string    `json:"target_path"`
	TotalSize   int64     `json:"total_size"`
	ChunkSize   int64     `json:"chunk_size"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Received    []int64   `json:"received"`
	ReceivedSum int64     `json:"received_sum"`
	CreatedAt   time.Time `json:"created_at"`
}

type chunkInfo struct {
	Index int
	Size  int64
}

type createUploadRequest struct {
	Filename   string `json:"filename"`
	TargetPath string `json:"target_path"`
	Size       int64  `json:"size"`
}

// uploadDir returns the on-disk session directory for an upload id. The id is
// a UUID, so the path is always contained under the data directory.
func (a *App) uploadDir(id uuid.UUID) string {
	return filepath.Join(a.cfg.DataDir, "uploads", id.String())
}

// isAbsDir reports whether path is an existing, absolute directory. Used by the
// admin-only endpoints that write to server folders.
func isAbsDir(path string) bool {
	// Guards recognized by CodeQL: only server-absolute paths without parent
	// references reach the filesystem call below.
	if path == "" || !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return false
	}
	st, err := os.Stat(filepath.Clean(path))
	return err == nil && st.IsDir()
}

func (a *App) loadUpload(id uuid.UUID) (uploadSession, error) {
	var u uploadSession
	err := a.pool.QueryRow(context.Background(),
		`SELECT id, filename, target_path, total_size, chunk_size, status, error, created_at
		 FROM uploads WHERE id=$1`, id).
		Scan(&u.ID, &u.Filename, &u.TargetPath, &u.TotalSize, &u.ChunkSize, &u.Status, &u.Error, &u.CreatedAt)
	return u, err
}

// receivedChunks lists the chunk indices currently stored for a session.
func receivedChunks(dir string) []chunkInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	chunks := []chunkInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".part") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".part"))
		if err != nil || idx < 0 {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		chunks = append(chunks, chunkInfo{Index: idx, Size: info.Size()})
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Index < chunks[j].Index })
	return chunks
}

func (u uploadSession) withChunks(dir string) uploadSession {
	if u.Status != "uploading" {
		return u
	}
	chunks := receivedChunks(dir)
	u.Received = make([]int64, 0, len(chunks))
	var sum int64
	for _, c := range chunks {
		u.Received = append(u.Received, int64(c.Index))
		sum += c.Size
	}
	u.ReceivedSum = sum
	return u
}

func (a *App) uploadSessionFromRow(u uploadSession) uploadSession {
	return u.withChunks(a.uploadDir(u.ID))
}

// GET /api/uploads — admin upload sessions (used to resume/cancel after a
// page refresh or network interruption).
func (a *App) listUploads(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, filename, target_path, total_size, chunk_size, status, error, created_at
		FROM uploads ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list uploads failed")
		return
	}
	defer rows.Close()

	items := []uploadSession{}
	for rows.Next() {
		var u uploadSession
		if err := rows.Scan(&u.ID, &u.Filename, &u.TargetPath, &u.TotalSize, &u.ChunkSize,
			&u.Status, &u.Error, &u.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan upload failed")
			return
		}
		items = append(items, a.uploadSessionFromRow(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/uploads — create a chunked upload session targeting a server
// directory. The client then PUTs numbered chunks and finishes with complete.
func (a *App) createUpload(w http.ResponseWriter, r *http.Request) {
	var req createUploadRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Filename = strings.TrimSpace(req.Filename)
	if req.Filename == "" {
		writeErr(w, http.StatusBadRequest, "filename is required")
		return
	}
	if strings.ContainsRune(req.Filename, '\x00') {
		writeErr(w, http.StatusBadRequest, "invalid filename")
		return
	}
	req.Filename = filepath.Base(req.Filename)
	if req.Filename == "." || req.Filename == ".." {
		writeErr(w, http.StatusBadRequest, "invalid filename")
		return
	}
	if !isAbsDir(req.TargetPath) {
		writeErr(w, http.StatusBadRequest, "target_path must be an existing absolute directory")
		return
	}
	if req.Size < 0 {
		writeErr(w, http.StatusBadRequest, "size must not be negative")
		return
	}

	var u uploadSession
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO uploads (filename, target_path, total_size, chunk_size)
		VALUES ($1, $2, $3, $4)
		RETURNING id, filename, target_path, total_size, chunk_size, status, error, created_at`,
		req.Filename, filepath.Clean(req.TargetPath), req.Size, defaultChunkSize).
		Scan(&u.ID, &u.Filename, &u.TargetPath, &u.TotalSize, &u.ChunkSize, &u.Status, &u.Error, &u.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create upload session failed")
		return
	}
	if err := os.MkdirAll(a.uploadDir(u.ID), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "create upload directory failed")
		return
	}
	writeJSON(w, http.StatusCreated, a.uploadSessionFromRow(u))
}

// GET /api/uploads/{id} — session status including already-received chunks.
func (a *App) getUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload id")
		return
	}
	u, err := a.loadUpload(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "upload session not found")
		return
	}
	writeJSON(w, http.StatusOK, a.uploadSessionFromRow(u))
}

// PUT /api/uploads/{id}/chunk/{index} — store one raw chunk. Idempotent:
// re-uploading the same index overwrites the previous chunk.
func (a *App) putChunk(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload id")
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		writeErr(w, http.StatusBadRequest, "invalid chunk index")
		return
	}
	u, err := a.loadUpload(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "upload session not found")
		return
	}
	if u.Status != "uploading" {
		writeErr(w, http.StatusConflict, "upload is not accepting chunks")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxChunkSize)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read chunk body failed")
		return
	}
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "chunk body is empty")
		return
	}
	if int64(len(data)) > u.ChunkSize {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("chunk exceeds %d bytes", u.ChunkSize))
		return
	}

	dir := a.uploadDir(u.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "upload directory unavailable")
		return
	}
	dst := filepath.Join(dir, strconv.Itoa(index)+".part")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "write chunk failed")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE uploads SET updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update upload failed")
		return
	}
	writeJSON(w, http.StatusOK, a.uploadSessionFromRow(u))
}

// POST /api/uploads/{id}/complete — verify and concatenate the chunks into the
// target directory. The library file watcher picks up the finished file when
// the target is inside a library.
func (a *App) completeUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload id")
		return
	}
	u, err := a.loadUpload(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "upload session not found")
		return
	}
	if u.Status != "uploading" {
		writeErr(w, http.StatusConflict, "upload is already finished")
		return
	}
	if !isAbsDir(u.TargetPath) {
		writeErr(w, http.StatusBadRequest, "target directory no longer exists")
		return
	}

	dir := a.uploadDir(u.ID)
	chunks := receivedChunks(dir)
	if len(chunks) == 0 {
		writeErr(w, http.StatusBadRequest, "no chunks uploaded yet")
		return
	}
	var gotSum int64
	for _, c := range chunks {
		gotSum += c.Size
	}
	if u.TotalSize > 0 {
		expected := (u.TotalSize + u.ChunkSize - 1) / u.ChunkSize
		if int64(len(chunks)) != expected || gotSum != u.TotalSize {
			writeErr(w, http.StatusBadRequest,
				fmt.Sprintf("incomplete upload: %d of %d chunks (%d of %d bytes)", len(chunks), expected, gotSum, u.TotalSize))
			return
		}
	} else {
		// Unknown size: require a contiguous chunk sequence starting at 0.
		for i, c := range chunks {
			if c.Index != i {
				writeErr(w, http.StatusBadRequest, "missing chunk "+strconv.Itoa(i))
				return
			}
		}
	}

	finalPath := filepath.Join(u.TargetPath, u.Filename)
	if _, err := os.Stat(finalPath); err == nil {
		writeErr(w, http.StatusConflict, "a file with this name already exists: "+u.Filename)
		return
	}

	tmp, err := os.CreateTemp(dir, ".final-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create temp file failed")
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	for _, c := range chunks {
		src, err := os.Open(filepath.Join(dir, strconv.Itoa(c.Index)+".part"))
		if err != nil {
			_ = tmp.Close()
			writeErr(w, http.StatusInternalServerError, "read chunk failed")
			return
		}
		_, copyErr := io.Copy(tmp, src)
		_ = src.Close()
		if copyErr != nil {
			_ = tmp.Close()
			writeErr(w, http.StatusInternalServerError, "assemble file failed")
			return
		}
	}
	if err := tmp.Close(); err != nil {
		writeErr(w, http.StatusInternalServerError, "finalize file failed")
		return
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		writeErr(w, http.StatusInternalServerError, "move file into target failed")
		return
	}

	u.Status = "completed"
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE uploads SET status='completed', error='', updated_at=now() WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "update upload failed")
		return
	}
	// Session chunks are no longer needed once the file is in place.
	_ = os.RemoveAll(dir)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          u.ID,
		"filename":    u.Filename,
		"target_path": u.TargetPath,
		"final_path":  finalPath,
		"status":      "completed",
	})
}

// DELETE /api/uploads/{id} — cancel the session and remove its chunks.
func (a *App) deleteUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload id")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `DELETE FROM uploads WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete upload failed")
		return
	}
	_ = os.RemoveAll(a.uploadDir(id))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
