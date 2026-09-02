package api

import (
	"net/http"
	"testing"
)

func TestSeriesSubscriptions(t *testing.T) {
	env := newIntegrationEnv(t)
	token := loginAdmin(t, env)
	libID := env.insertLibrary(t, false)
	seriesID, _ := env.insertSeriesVideo(t, libID, "Channel Show", 1, 1)

	status, d := doJSON(t, http.MethodPut,
		env.server.URL+"/api/series/"+seriesID.String()+"/subscribe", token, nil)
	if status != http.StatusOK {
		t.Fatalf("subscribe status = %d body = %v", status, d)
	}

	status, d = doJSON(t, http.MethodGet,
		env.server.URL+"/api/series/"+seriesID.String(), token, nil)
	if status != http.StatusOK {
		t.Fatalf("series detail status = %d", status)
	}
	series := d["series"].(map[string]any)
	if series["is_subscribed"] != true || int(series["subscribers"].(float64)) != 1 {
		t.Fatalf("series = %v", series)
	}

	status, d = doJSON(t, http.MethodGet,
		env.server.URL+"/api/users/me/subscriptions", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 1 {
		t.Fatalf("subscriptions status = %d body = %v", status, d)
	}

	status, _ = doJSON(t, http.MethodDelete,
		env.server.URL+"/api/series/"+seriesID.String()+"/subscribe", token, nil)
	if status != http.StatusOK {
		t.Fatalf("unsubscribe status = %d", status)
	}
	status, d = doJSON(t, http.MethodGet,
		env.server.URL+"/api/users/me/subscriptions", token, nil)
	if status != http.StatusOK || len(d["items"].([]any)) != 0 {
		t.Fatalf("subscriptions after unsubscribe = %v", d)
	}
}
