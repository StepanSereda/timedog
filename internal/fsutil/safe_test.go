package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Гарантии: логические пути из отчёта не должны выходить за пределы корня снимка (path traversal).

func TestResolveUnderRoot_acceptsNormalPaths(t *testing.T) {
	root := filepath.Clean("/Volumes/TM/Backups.backupdb/host/2024-01-01")
	tests := []struct {
		logical string
		wantSuffix string // фрагмент конца ожидаемого абсолютного пути (после root)
	}{
		{"/Macintosh HD/Users/x/file.txt", "Macintosh HD/Users/x/file.txt"},
		{"Macintosh HD/Users/x/file.txt", "Macintosh HD/Users/x/file.txt"},
		{"/Data/", "Data"},
	}
	for _, tt := range tests {
		t.Run(tt.logical, func(t *testing.T) {
			got, err := ResolveUnderRoot(root, tt.logical)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(got, root) {
				t.Fatalf("result %q does not start with root %q", got, root)
			}
			if !strings.HasSuffix(got, filepath.FromSlash(tt.wantSuffix)) {
				t.Fatalf("got %q, want suffix %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestResolveUnderRoot_rejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "snapshot")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmp, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	// Подняться из snapshot в родителя и зайти в соседний каталог.
	relUp := ".." + string(filepath.Separator) + filepath.Base(outside) + string(filepath.Separator) + "secret"
	logical := "/" + filepath.ToSlash(relUp)

	_, err := ResolveUnderRoot(root, logical)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escape error, got: %v", err)
	}
}

func TestResolveUnderRoot_plainDotDotFileName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snap")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Имя сегмента «..» внутри корня — допустимо, если не выходит за root.
	got, err := ResolveUnderRoot(root, "/foo/../bar")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "bar")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIsSubpath_prefixTrap(t *testing.T) {
	// Регрессия: /tmp/foobar не должен считаться вложенным в /tmp/foo
	root := filepath.Clean("/tmp/foo")
	abs := filepath.Clean("/tmp/foobar/baz")
	if isSubpath(abs, root) {
		t.Fatalf("/tmp/foobar/baz must not be under /tmp/foo")
	}
}
