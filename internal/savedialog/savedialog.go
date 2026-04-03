// Package savedialog opens a native «Save file» dialog when the app runs on the same machine as the user (localhost).
package savedialog

import (
	"errors"
	"regexp"
	"strings"
)

// ErrCanceled is returned when the user closes the dialog without saving.
var ErrCanceled = errors.New("save dialog canceled")

// ErrUnavailable means this OS has no supported implementation (use manual path entry).
var ErrUnavailable = errors.New("native save dialog not available")

var fileNameSafe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// SanitizeSuggestedName keeps a safe default filename for dialog APIs.
func SanitizeSuggestedName(s string) string {
	s = strings.TrimSpace(s)
	base := s
	if i := strings.LastIndex(s, "/"); i >= 0 {
		base = s[i+1:]
	}
	if i := strings.LastIndex(base, "\\"); i >= 0 {
		base = base[i+1:]
	}
	if base == "" || !fileNameSafe.MatchString(base) {
		return "timedog-report.jsonl"
	}
	low := strings.ToLower(base)
	if strings.HasSuffix(low, ".jsonl.gz") {
		return base
	}
	if strings.HasSuffix(low, ".jsonl") {
		return base
	}
	return base + ".jsonl"
}
