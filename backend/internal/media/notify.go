package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Notifier fans events out to configured channels (webhook, Apprise).
type Notifier struct {
	WebhookURL string
	AppriseURL string
	client     *http.Client
}

func NewNotifier(webhookURL, appriseURL string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		AppriseURL: appriseURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers an event payload to every configured channel. Errors are only
// logged: notifications must never break the main flow.
func (n *Notifier) Send(ctx context.Context, event, title, body string, data map[string]any) {
	payload := map[string]any{
		"event": event,
		"title": title,
		"body":  body,
		"time":  time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range data {
		payload[k] = v
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notify marshal: %v", err)
		return
	}
	if n.WebhookURL != "" {
		if err := n.postJSON(ctx, n.WebhookURL, raw); err != nil {
			log.Printf("notify webhook: %v", err)
		}
	}
	if n.AppriseURL != "" {
		apprise := map[string]string{"title": title, "body": body, "type": "info"}
		araw, _ := json.Marshal(apprise)
		if err := n.postJSON(ctx, n.AppriseURL, araw); err != nil {
			log.Printf("notify apprise: %v", err)
		}
	}
}

func (n *Notifier) postJSON(ctx context.Context, url string, raw []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}
