package api

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewShareToken(t *testing.T) {
	tok, err := newShareToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 48 {
		t.Errorf("token length = %d, want 48 hex chars", len(tok))
	}
	if _, err := hex.DecodeString(tok); err != nil {
		t.Errorf("token is not hex: %v", err)
	}
	other, _ := newShareToken()
	if tok == other {
		t.Error("two generated tokens must differ")
	}
}

func TestSrtToVTT(t *testing.T) {
	in := "1\r\n00:00:01,000 --> 00:00:03,500\r\nHello\r\n\r\n"
	out := string(srtToVTT([]byte(in)))
	if !strings.HasPrefix(out, "WEBVTT") {
		t.Errorf("output missing WEBVTT header: %q", out)
	}
	if !strings.Contains(out, "00:00:01.000 --> 00:00:03.500") {
		t.Errorf("timestamps not converted: %q", out)
	}
}

func TestBuildSubtitlePlaylist(t *testing.T) {
	p := string(buildSubtitlePlaylist("abc"))
	if !strings.Contains(p, "subtitle.vtt?token=abc") {
		t.Errorf("subtitle playlist missing VTT URI: %q", p)
	}
}
