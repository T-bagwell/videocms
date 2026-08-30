package media

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer speaks just enough SMTP to capture one DATA payload.
type fakeSMTPServer struct {
	mu      sync.Mutex
	message string
	host    string
	port    int
	ln      net.Listener
}

func startFakeSMTP(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTPServer{ln: ln}
	addr := ln.Addr().(*net.TCPAddr)
	s.host = addr.IP.String()
	s.port = ln.Addr().(*net.TCPAddr).Port
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	write("220 fake ESMTP")
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case inData:
			if cmd == "." {
				inData = false
				write("250 OK")
				continue
			}
			s.mu.Lock()
			s.message += line
			s.mu.Unlock()
		case strings.HasPrefix(cmd, "EHLO"):
			write("250-fake\r\n250 OK")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO"):
			write("250 OK")
		case strings.HasPrefix(cmd, "DATA"):
			inData = true
			write("354 go ahead")
		case cmd == "QUIT":
			write("221 bye")
			return
		case cmd == "RSET":
			write("250 OK")
		default:
			write("250 OK")
		}
	}
}

func TestNotifierSendsSMTP(t *testing.T) {
	srv := startFakeSMTP(t)
	n := NewNotifier("", "", SMTPConfig{
		Host: srv.host,
		Port: srv.port,
		From: "videocms@example.com",
		To:   []string{"ops@example.com", "dev@example.com"},
	})
	if err := n.sendSMTP(context.Background(), "Scan done", "Library Movies finished with status ok"); err != nil {
		t.Fatalf("sendSMTP: %v", err)
	}
	srv.mu.Lock()
	msg := srv.message
	srv.mu.Unlock()
	for _, want := range []string{
		"From: videocms@example.com",
		"To: ops@example.com, dev@example.com",
		"Subject: Scan done",
		"Library Movies finished with status ok",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestNotifierSMTPDisabled(t *testing.T) {
	n := NewNotifier("", "", SMTPConfig{})
	n.Send(context.Background(), "test", "title", "body", nil) // must not panic or block
}

func TestBuildEmailHeaders(t *testing.T) {
	msg := string(buildEmail("a@example.com", []string{"b@example.com"}, "你好 VideoCMS", "body line"))
	for _, want := range []string{
		"From: a@example.com",
		"To: b@example.com",
		"Content-Type: text/plain; charset=UTF-8",
		"body line",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %s", want, msg)
		}
	}
	if !strings.Contains(strings.ToLower(msg), "=?utf-8?q?") {
		t.Fatalf("expected encoded subject, got: %s", msg)
	}
}
