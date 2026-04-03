package scan

import (
	"testing"

	"timedog/internal/report"
)

// Гарантии: режим -d даёт ожидаемое число строк и корректную агрегацию в родительский каталог.

func TestRollupByDepth_mergesDeepPaths(t *testing.T) {
	d := 2
	opts := Options{SortBy: 2}
	entries := []report.Entry{
		{Path: "/Vol/Users/x/a.txt", OldSize: 1, NewSize: 10},  // глубина 3 -> в /Vol/Users/
		{Path: "/Vol/Users/x/b.txt", OldSize: 2, NewSize: 20},
		{Path: "/Vol/Lib/file", OldSize: 0, NewSize: 5}, // глубина 3 -> в /Vol/Lib/
		{Path: "/Vol/README", OldSize: 1, NewSize: 1},   // глубина 2, файл — не rollup
	}
	out := rollupByDepth(entries, d, opts)

	var usersRoll, libRoll, readme *report.Entry
	for i := range out {
		e := &out[i]
		switch e.Path {
		case "/Vol/Users/":
			usersRoll = e
		case "/Vol/Lib/":
			libRoll = e
		case "/Vol/README":
			readme = e
		}
	}
	if usersRoll == nil || libRoll == nil || readme == nil {
		t.Fatalf("missing expected rows: %#v", out)
	}
	if !usersRoll.IsDir || usersRoll.NewSize != 30 || usersRoll.OldSize != 3 {
		t.Fatalf("Users rollup: %+v", usersRoll)
	}
	if usersRoll.InDir != 2 {
		t.Fatalf("Users InDir: %d", usersRoll.InDir)
	}
	if libRoll.NewSize != 5 || libRoll.InDir != 1 {
		t.Fatalf("Lib rollup: %+v", libRoll)
	}
}

func TestRollupByDepth_noOpWhenDepthZero(t *testing.T) {
	in := []report.Entry{{Path: "/a", NewSize: 1}}
	out := rollupByDepth(in, 0, Options{})
	if len(out) != 1 || out[0].Path != "/a" {
		t.Fatal(out)
	}
}

func TestRollupByDepth_respectsMinSizeOnBucket(t *testing.T) {
	min := int64(100)
	opts := Options{SortBy: 2, MinSizeBytes: &min}
	entries := []report.Entry{
		{Path: "/a/b/x", NewSize: 10},
		{Path: "/a/b/y", NewSize: 20},
	}
	out := rollupByDepth(entries, 1, opts)
	for _, e := range out {
		if e.Path == "/a/" {
			t.Fatalf("bucket /a/ should be dropped: new sum 30 < min 100, got %+v", e)
		}
	}
}

func TestPathSegments_trimsSymlinkMarker(t *testing.T) {
	p := pathSegments("/foo/bar@")
	if len(p) != 2 || p[1] != "bar" {
		t.Fatalf("%v", p)
	}
}
