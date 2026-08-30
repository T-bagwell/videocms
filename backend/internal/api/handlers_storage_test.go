package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestStoragePoolsAndPoolTarget(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	status, d := doJSON(t, "POST", env.server.URL+"/api/admin/storage-pools", token,
		map[string]any{"name": "media", "type": "local", "mount_path": dir, "config": map[string]any{}, "readonly": false})
	if status != http.StatusCreated {
		t.Fatalf("create pool status = %d, body = %v", status, d)
	}
	poolID := d["id"].(string)

	status, d = doJSON(t, "GET", env.server.URL+"/api/admin/storage-pools", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list pools status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("pool count = %d, want 1", len(items))
	}

	// Uploads accept pool:// references that resolve to the mount path.
	status, d = doJSON(t, "POST", env.server.URL+"/api/uploads", token,
		map[string]any{"filename": "a.mp4", "target_path": "pool://media/sub", "size": 1})
	if status != http.StatusCreated {
		t.Fatalf("pool upload status = %d, body = %v", status, d)
	}
	if d["target_path"] != sub {
		t.Fatalf("resolved target = %v, want %s", d["target_path"], sub)
	}

	status, _ = doJSON(t, "GET", env.server.URL+"/api/storage-pools", token, nil)
	if status != http.StatusOK {
		t.Fatalf("user pools status = %d", status)
	}

	status, _ = doJSON(t, "DELETE", env.server.URL+"/api/admin/storage-pools/"+poolID, token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete pool status = %d", status)
	}
}
