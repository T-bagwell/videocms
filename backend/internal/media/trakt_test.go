package media

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTraktSyncHistoryDedupes(t *testing.T) {
	now := time.Now()
	payload, count := buildTraktPayload([]TraktItem{
		{TmdbID: 1, WatchedAt: time.Now()},
		{TmdbID: 1, WatchedAt: now}, // duplicate: latest wins
		{TmdbID: 2, WatchedAt: now.Add(time.Hour)},
		{TmdbID: 0, WatchedAt: now}, // ignored
	})
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	data, _ := json.Marshal(payload)
	if len(data) == 0 {
		t.Fatal("empty payload")
	}
}
