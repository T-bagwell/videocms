package api

import (
	"context"
	"net/http"
	"testing"
)

func TestLibraryNFOExportImport(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	videoID := env.insertVideo(t, libID, "Old Title", ".mkv")

	status, d := doJSON(t, "POST", env.server.URL+"/api/libraries/"+libID.String()+"/export-nfo", token, nil)
	if status != http.StatusOK {
		t.Fatalf("export nfo status = %d, body = %v", status, d)
	}
	if d["exported"].(float64) != 1 {
		t.Fatalf("exported = %v, want 1", d["exported"])
	}

	// Corrupt the DB row, then import the NFO back.
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE videos SET title='Wrong', synopsis='' WHERE id=$1`, videoID); err != nil {
		t.Fatal(err)
	}
	status, d = doJSON(t, "POST", env.server.URL+"/api/libraries/"+libID.String()+"/import-nfo", token, nil)
	if status != http.StatusOK {
		t.Fatalf("import nfo status = %d, body = %v", status, d)
	}
	if d["updated"].(float64) != 1 {
		t.Fatalf("updated = %v, want 1", d["updated"])
	}
	status, d = doJSON(t, "GET", env.server.URL+"/api/videos/"+videoID.String(), token, nil)
	if status != http.StatusOK || d["title"] != "Old Title" {
		t.Fatalf("video after import status = %d, body = %v", status, d)
	}
}
