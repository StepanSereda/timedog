package report

import "time"

const KindMeta = "timedog-report-meta"
const KindEntry = "entry"

// SkippedPath is a path that could not be read during scan (access, lstat, etc.).
type SkippedPath struct {
	Path   string `json:"path"`   // logical path under snapshot root
	Reason string `json:"reason"` // walk | info | lstat | lstat_access
}

// Meta is the first line of a JSONL report.
type Meta struct {
	Kind             string            `json:"kind"`
	V                int               `json:"v"`
	OldRoot          string            `json:"old_root"`
	NewRoot          string            `json:"new_root"`
	OldLabel         string            `json:"old_label"`
	NewLabel         string            `json:"new_label"`
	CreatedAt        time.Time         `json:"created_at"`
	Options          map[string]any    `json:"options,omitempty"`
	Totals           *Totals           `json:"totals,omitempty"`
	Skipped          []SkippedPath     `json:"skipped,omitempty"`           // sample, see SkippedTotal
	SkippedTotal     int               `json:"skipped_total,omitempty"`     // all skip events
	SkippedTruncated bool              `json:"skipped_truncated,omitempty"` // true if len(Skipped) < SkippedTotal
}

// Totals aggregates after scan.
type Totals struct {
	ChangedFiles int   `json:"changed_files"`
	SizeBytes    int64 `json:"size_bytes"`
}

// Entry is one changed path (one JSONL line after meta).
type Entry struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	OldSize     int64  `json:"old_size"`
	NewSize     int64  `json:"new_size"`
	OldStr      string `json:"old_str"`
	NewStr      string `json:"new_str"`
	IsDir       bool   `json:"is_dir"`
	IsSymlink   bool   `json:"is_symlink"`
	UnknownOld  bool   `json:"unknown_old"`
	InDir       int    `json:"in_dir,omitempty"` // timedog -d count-1 in brackets
}
