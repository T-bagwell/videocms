package media

import (
	"fmt"
	"testing"
)

func TestParseProgress(t *testing.T) {
	cases := []struct {
		line string
		want float64
		ok   bool
	}{
		{"[download]  45.2% of 100.00MiB at 2.3MiB/s ETA 00:24", 45.2, true},
		{"[download] 100% of 1.00MiB", 100, true},
		{"[download] Destination: foo.mp4", 0, false},
		{"[info] something", 0, false},
	}
	for i, c := range cases {
		got, ok := parseProgress(c.line)
		if ok != c.ok || (ok && fmt.Sprintf("%.1f", got) != fmt.Sprintf("%.1f", c.want)) {
			t.Errorf("case %d: parseProgress(%q) = %v, %v; want %v, %v", i, c.line, got, ok, c.want, c.ok)
		}
	}
}
