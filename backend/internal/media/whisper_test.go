package media

import "testing"

func TestVTTText(t *testing.T) {
	in := []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello transcript\n\n00:00:02.500 --> 00:00:03.000\nSecond line\n")
	got := VTTText(in)
	want := "Hello transcript Second line"
	if got != want {
		t.Fatalf("VTTText = %q, want %q", got, want)
	}
}
