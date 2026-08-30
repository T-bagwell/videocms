package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type storagePool struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	MountPath string          `json:"mount_path"`
	Config    json.RawMessage `json:"config"`
	Readonly  bool            `json:"readonly"`
	CreatedAt time.Time       `json:"created_at"`
}

type storagePoolRequest struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	MountPath string          `json:"mount_path"`
	Config    json.RawMessage `json:"config"`
	Readonly  bool            `json:"readonly"`
}

func (a *App) poolFromRow(row interface{ Scan(...any) error }) (storagePool, error) {
	var p storagePool
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.MountPath, &p.Config, &p.Readonly, &p.CreatedAt)
	return p, err
}

func validatePoolReq(req *storagePoolRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.MountPath = strings.TrimSpace(req.MountPath)
	if req.Name == "" {
		return "name is required"
	}
	if req.Type != "local" && req.Type != "s3" && req.Type != "sftp" {
		return "type must be local, s3 or sftp"
	}
	if req.MountPath == "" || !filepath.IsAbs(req.MountPath) {
		return "mount_path must be an absolute local path (use a mounted S3/SFTP folder for remote pools)"
	}
	if len(req.Config) == 0 {
		req.Config = json.RawMessage(`{}`)
	}
	return ""
}

// GET /api/admin/storage-pools — full pool list (admin).
func (a *App) listStoragePoolsAdmin(w http.ResponseWriter, r *http.Request) {
	a.listStoragePools(w, r, true)
}

// GET /api/storage-pools — safe pool list for path pickers (user).
func (a *App) listStoragePoolsUser(w http.ResponseWriter, r *http.Request) {
	a.listStoragePools(w, r, false)
}

func (a *App) listStoragePools(w http.ResponseWriter, r *http.Request, admin bool) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, type, mount_path, config, readonly, created_at
		FROM storage_pools ORDER BY name`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list storage pools failed")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		p, err := a.poolFromRow(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan storage pool failed")
			return
		}
		if admin {
			items = append(items, map[string]any{"id": p.ID, "name": p.Name, "type": p.Type,
				"mount_path": p.MountPath, "config": json.RawMessage(p.Config), "readonly": p.Readonly})
		} else {
			items = append(items, map[string]any{"name": p.Name, "type": p.Type,
				"mount_path": p.MountPath, "readonly": p.Readonly})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/storage-pools
func (a *App) createStoragePool(w http.ResponseWriter, r *http.Request) {
	var req storagePoolRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if msg := validatePoolReq(&req); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	var p storagePool
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO storage_pools (name, type, mount_path, config, readonly)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, type, mount_path, config, readonly, created_at`,
		req.Name, req.Type, req.MountPath, req.Config, req.Readonly).Scan(
		&p.ID, &p.Name, &p.Type, &p.MountPath, &p.Config, &p.Readonly, &p.CreatedAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "create storage pool failed")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// PATCH /api/admin/storage-pools/{id}
func (a *App) updateStoragePool(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid pool id")
		return
	}
	var req storagePoolRequest
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if msg := validatePoolReq(&req); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	var p storagePool
	if err := a.pool.QueryRow(r.Context(), `
		UPDATE storage_pools SET name=$1, type=$2, mount_path=$3, config=$4, readonly=$5
		WHERE id=$6
		RETURNING id, name, type, mount_path, config, readonly, created_at`,
		req.Name, req.Type, req.MountPath, req.Config, req.Readonly, id).Scan(
		&p.ID, &p.Name, &p.Type, &p.MountPath, &p.Config, &p.Readonly, &p.CreatedAt); err != nil {
		writeErr(w, http.StatusNotFound, "storage pool not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// DELETE /api/admin/storage-pools/{id}
func (a *App) deleteStoragePool(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid pool id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM storage_pools WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete storage pool failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// resolveTargetPath turns a pool://<name>/<sub> reference into the pool's
// mount path (joined with the optional sub path); plain paths pass through.
func (a *App) resolveTargetPath(ref string) (string, bool) {
	if !strings.HasPrefix(ref, "pool://") {
		return ref, true
	}
	rest := strings.TrimPrefix(ref, "pool://")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]
	var mount string
	if err := a.pool.QueryRow(context.Background(),
		`SELECT mount_path FROM storage_pools WHERE name=$1 AND readonly=false`, name).Scan(&mount); err != nil {
		return "", false
	}
	if len(parts) == 2 && parts[1] != "" {
		return filepath.Join(mount, parts[1]), true
	}
	return mount, true
}
