package api

import (
	"io"
	"net/http"
	"testing"
)

func TestMaintenanceRunAndBackups(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)

	status, d := doJSON(t, "POST", env.server.URL+"/api/admin/maintenance/run", token, nil)
	if status != http.StatusOK {
		t.Fatalf("maintenance run status = %d, body = %v", status, d)
	}
	backup, _ := d["backup"].(string)
	if backup == "" {
		t.Fatalf("maintenance summary missing backup: %v", d)
	}

	status, d = doJSON(t, "GET", env.server.URL+"/api/admin/backups", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list backups status = %d", status)
	}
	items, _ := d["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no backups listed after maintenance")
	}
	name := items[0].(map[string]any)["name"].(string)

	req, _ := http.NewRequest("GET", env.server.URL+"/api/admin/backups/"+name, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("download backup status = %d: %s", res.StatusCode, b)
	}
}
