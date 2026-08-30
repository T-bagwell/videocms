package media

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestDecodeSubtitlePayload(t *testing.T) {
	plain := []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n")

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(plain)
	_ = w.Close()

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	f, _ := zw.Create("movie.en.srt")
	_, _ = f.Write(plain)
	_ = zw.Close()

	for name, in := range map[string][]byte{
		"plain": plain,
		"gzip":  gz.Bytes(),
		"zip":   zbuf.Bytes(),
	} {
		out, err := decodeSubtitlePayload(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Contains(out, []byte("00:00:01,000")) {
			t.Fatalf("%s: output missing subtitle content: %q", name, out)
		}
	}
}
