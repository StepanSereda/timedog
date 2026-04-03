package scan

import (
	"path/filepath"
	"strings"

	"timedog/internal/report"
)

func pathSegments(p string) []string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	p = strings.TrimSuffix(p, "@")
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	var parts []string
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			parts = append(parts, x)
		}
	}
	return parts
}

func dirKeyFromParts(parts []string, depth int) string {
	if depth <= 0 {
		return "/"
	}
	if len(parts) < depth {
		return "/" + strings.Join(parts, "/") + "/"
	}
	return "/" + strings.Join(parts[:depth], "/") + "/"
}

// rollupByDepth merges entries deeper than depth (and directories exactly at depth)
// into one summary row per ancestor directory at the given segment depth — same idea as timedog -d.
func rollupByDepth(entries []report.Entry, depth int, opts Options) []report.Entry {
	if depth <= 0 {
		return entries
	}

	type acc struct {
		oldSum     int64
		newSum     int64
		unknownOld bool
		rcnt       int
	}
	buckets := map[string]*acc{}
	var direct []report.Entry

	for i := range entries {
		e := entries[i]
		parts := pathSegments(e.Path)
		if len(parts) == 0 {
			continue
		}
		segs := len(parts)
		roll := segs > depth || (e.IsDir && segs == depth)
		if !roll {
			direct = append(direct, e)
			continue
		}
		key := dirKeyFromParts(parts, depth)
		b := buckets[key]
		if b == nil {
			b = &acc{}
			buckets[key] = b
		}
		b.oldSum += e.OldSize
		b.newSum += e.NewSize
		if e.UnknownOld {
			b.unknownOld = true
		}
		b.rcnt++
	}

	out := make([]report.Entry, 0, len(direct)+len(buckets))
	out = append(out, direct...)

	for key, b := range buckets {
		if b.newSum == 0 && b.oldSum == 0 && !b.unknownOld {
			continue
		}
		if opts.MinSizeBytes != nil && b.newSum < *opts.MinSizeBytes {
			continue
		}
		out = append(out, report.Entry{
			Path:       key,
			OldSize:    b.oldSum,
			NewSize:    b.newSum,
			OldStr:     sizeStr(b.oldSum, b.unknownOld, opts),
			NewStr:     report.FormatDisplay(b.newSum, opts.UseBase10, opts.SimpleFormat),
			IsDir:      true,
			IsSymlink:  false,
			UnknownOld: b.unknownOld,
			InDir:      b.rcnt,
		})
	}

	sortEntries(out, opts.SortBy)
	return out
}
