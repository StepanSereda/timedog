package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_fastWalkMatchesSequential(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	oldRoot := filepath.Join(base, "old")
	newRoot := filepath.Join(base, "new")
	if err := os.MkdirAll(oldRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "f.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "f.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{SortBy: 2}

	rSeq, err := RunSequential(ctx, oldRoot, newRoot, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	rFast, err := Run(ctx, oldRoot, newRoot, opts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(rSeq.Entries) != len(rFast.Entries) {
		t.Fatalf("len seq=%d fast=%d", len(rSeq.Entries), len(rFast.Entries))
	}
	if rSeq.Totals != rFast.Totals {
		t.Fatalf("totals seq=%+v fast=%+v", rSeq.Totals, rFast.Totals)
	}
	if rSeq.SkippedTotal != rFast.SkippedTotal {
		t.Fatalf("skipped total seq=%d fast=%d", rSeq.SkippedTotal, rFast.SkippedTotal)
	}
	for i := range rSeq.Entries {
		if rSeq.Entries[i].Path != rFast.Entries[i].Path {
			t.Fatalf("entry %d path seq=%q fast=%q", i, rSeq.Entries[i].Path, rFast.Entries[i].Path)
		}
		if rSeq.Entries[i].NewSize != rFast.Entries[i].NewSize {
			t.Fatalf("entry %d size", i)
		}
	}
}
