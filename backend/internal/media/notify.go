package media

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig describes the mail channel used by the Notifier.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	To       []string
}

// Notifier fans events out to configured channels (webhook, Apprise, email).
type Notifier struct {
	WebhookURL string
	AppriseURL string
	SMTP       SMTPConfig
	client     *http.Client
}

func NewNotifier(webhookURL, appriseURL string, smtp SMTPConfig) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		AppriseURL: appriseURL,
		SMTP:       smtp,
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
	if n.SMTP.Host != "" && n.SMTP.From != "" && len(n.SMTP.To) > 0 {
		if err := n.sendSMTP(ctx, title, body); err != nil {
			log.Printf("notify smtp: %v", err)
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

// sendSMTP delivers a plain-text notification via SMTP, supporting implicit
// TLS (port 465), STARTTLS (587/25) and optional PLAIN authentication.
func (n *Notifier) sendSMTP(ctx context.Context, title, body string) error {
	cfg := n.SMTP
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()
	if port == 465 {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: cfg.Host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("tls handshake: %w", err)
		}
		conn = tlsConn
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = c.Close() }()
	if port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, to := range cfg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("rcpt %s: %w", to, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(buildEmail(cfg.From, cfg.To, title, body)); err != nil {
		_ = w.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finish message: %w", err)
	}
	return c.Quit()
}

// buildEmail assembles a minimal RFC 5322 message with UTF-8 headers.
func buildEmail(from string, to []string, title, body string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", title))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&b, "\r\n%s\r\n", body)
	return b.Bytes()
}
