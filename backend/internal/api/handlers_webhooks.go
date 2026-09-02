package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type webhookSubscription struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// GET /api/admin/webhooks
func (a *App) listWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, url, secret, events, active, created_at
		FROM webhook_subscriptions ORDER BY created_at`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list webhooks failed")
		return
	}
	defer rows.Close()
	items := []webhookSubscription{}
	for rows.Next() {
		var s webhookSubscription
		if err := rows.Scan(&s.ID, &s.URL, &s.Secret, &s.Events, &s.Active, &s.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan webhook failed")
			return
		}
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/admin/webhooks
func (a *App) createWebhook(w http.ResponseWriter, r *http.Request) {
	var req webhookSubscription
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" || (!strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "http://")) {
		writeErr(w, http.StatusBadRequest, "url must start with http:// or https://")
		return
	}
	if len(req.Events) == 0 {
		req.Events = []string{}
	}
	var s webhookSubscription
	if err := a.pool.QueryRow(r.Context(), `
		INSERT INTO webhook_subscriptions (url, secret, events, active)
		VALUES ($1, $2, $3, $4)
		RETURNING id, url, secret, events, active, created_at`,
		req.URL, req.Secret, req.Events, req.Active).Scan(
		&s.ID, &s.URL, &s.Secret, &s.Events, &s.Active, &s.CreatedAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "create webhook failed")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

// PATCH /api/admin/webhooks/{id}
func (a *App) updateWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid webhook id")
		return
	}
	var req webhookSubscription
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var s webhookSubscription
	if err := a.pool.QueryRow(r.Context(), `
		UPDATE webhook_subscriptions SET url=$1, secret=$2, events=$3, active=$4
		WHERE id=$5
		RETURNING id, url, secret, events, active, created_at`,
		req.URL, req.Secret, req.Events, req.Active, id).Scan(
		&s.ID, &s.URL, &s.Secret, &s.Events, &s.Active, &s.CreatedAt); err != nil {
		writeErr(w, http.StatusNotFound, "webhook not found")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// DELETE /api/admin/webhooks/{id}
func (a *App) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid webhook id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM webhook_subscriptions WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete webhook failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// notifyEvent delivers to the configured channels plus every active
// subscription whose event filter matches, with an HMAC signature.
func (a *App) notifyEvent(event, title, body string, data map[string]any) {
	payload := map[string]any{"event": event, "title": title, "body": body,
		"time": time.Now().UTC().Format(time.RFC3339)}
	for k, v := range data {
		payload[k] = v
	}
	raw, _ := json.Marshal(payload)
	a.notify.Send(context.Background(), event, title, body, data)
	a.deliverPluginEvents(raw, event)

	rows, err := a.pool.Query(context.Background(), `
		SELECT id, url, secret, events FROM webhook_subscriptions WHERE active=true`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var url, secret string
		var events []string
		if err := rows.Scan(&id, &url, &secret, &events); err != nil {
			continue
		}
		if len(events) > 0 && !containsString(events, event) {
			continue
		}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(string(raw)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(raw)
			req.Header.Set("X-Videocms-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		client := &http.Client{Timeout: 10 * time.Second}
		res, err := client.Do(req)
		if err == nil {
			_ = res.Body.Close()
		}
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
