package media

import "regexp"

var segNameRe = regexp.MustCompile(`^seg_\d+\.ts$`)

// SegmentNameMatch reports whether name looks like an HLS segment filename.
func SegmentNameMatch(name string) bool {
	return segNameRe.MatchString(name)
}

