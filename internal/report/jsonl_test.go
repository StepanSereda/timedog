package report

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Гарантии: отчёт можно записать и снова прочитать; gzip прозрачен; служебные строки не ломают импорт.

func TestWriteJSONL_roundTripPlain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.jsonl")

	meta := Meta{
		V:        1,
		OldRoot:  "/old/snap",
		NewRoot:  "/new/snap",
		OldLabel: "a",
		NewLabel: "b",
		CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	entries := []Entry{
		{Path: "/f1", OldSize: 0, NewSize: 10, OldStr: "0B", NewStr: "10B", IsDir: false},
		{Path: "/d/", OldSize: 1, NewSize: 2, OldStr: "1B", NewStr: "2B", IsDir: true},
	}

	if err := WriteJSONL(path, meta, entries); err != nil {
		t.Fatal(err)
	}

	gotMeta, gotEntries, err := ParseJSONLFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.Kind != KindMeta {
		t.Fatalf("meta kind: got %q", gotMeta.Kind)
	}
	if gotMeta.OldRoot != meta.OldRoot || gotMeta.NewRoot != meta.NewRoot {
		t.Fatalf("meta roots: %+v", gotMeta)
	}
	if len(gotEntries) != 2 {
		t.Fatalf("entries len: %d", len(gotEntries))
	}
	if gotEntries[0].Path != "/f1" || gotEntries[1].Path != "/d/" {
		t.Fatalf("paths: %#v", gotEntries)
	}
}

func TestWriteJSONL_roundTripGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.jsonl.gz")

	meta := Meta{V: 1, OldRoot: "/o", NewRoot: "/n"}
	entries := []Entry{{Path: "/x", NewSize: 5}}

	if err := WriteJSONL(path, meta, entries); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gotMeta, gotEntries, err := ParseJSONL(f)
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.OldRoot != "/o" || len(gotEntries) != 1 || gotEntries[0].Path != "/x" {
		t.Fatalf("got meta %+v entries %#v", gotMeta, gotEntries)
	}
}

func TestParseJSONL_skipsSummaryLines(t *testing.T) {
	raw := `{"kind":"timedog-report-meta","v":1,"old_root":"/a","new_root":"/b"}
{"kind":"entry","path":"/p","old_size":1,"new_size":2}
{"kind":"summary","totals":{}}
`
	gotMeta, gotEntries, err := ParseJSONL(bytes.NewReader([]byte(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.V != 1 {
		t.Fatal(gotMeta)
	}
	if len(gotEntries) != 1 || gotEntries[0].Path != "/p" {
		t.Fatalf("%#v", gotEntries)
	}
}

func TestParseJSONL_emptyErrors(t *testing.T) {
	_, _, err := ParseJSONL(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseJSONL_metaOnlyStillValid(t *testing.T) {
	raw := `{"kind":"timedog-report-meta","v":1,"old_root":"/x","new_root":"/y"}
`
	meta, entries, err := ParseJSONL(bytes.NewReader([]byte(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if meta.NewRoot != "/y" || len(entries) != 0 {
		t.Fatalf("meta=%+v entries=%d", meta, len(entries))
	}
}

func TestLabelsFromRoots(t *testing.T) {
	o, n := LabelsFromRoots("/vol/Latest", "/vol/2024-01-01-120000")
	if o != "Latest" || n != "2024-01-01-120000" {
		t.Fatalf("got %q %q", o, n)
	}
}

func TestWriteJSONL_setsRequiredKinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.jsonl")
	meta := Meta{OldRoot: "/a", NewRoot: "/b"}
	if err := WriteJSONL(path, meta, []Entry{{Path: "/z"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("lines: %s", string(data))
	}
	if !bytes.Contains(lines[0], []byte(KindMeta)) {
		t.Fatalf("first line should be meta: %s", lines[0])
	}
	if !bytes.Contains(lines[1], []byte(KindEntry)) {
		t.Fatalf("second line should be entry: %s", lines[1])
	}
}

func TestStreamReportWriter_finalWriteJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live.jsonl")
	ts := time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)
	partial := Meta{V: 1, OldRoot: "/o", NewRoot: "/n", CreatedAt: ts}
	sw, err := NewStreamReportWriter(path, partial)
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.WriteEntry(Entry{Path: "/unsorted", NewSize: 3}); err != nil {
		t.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}

	fullMeta := Meta{
		V:         1,
		OldRoot:   "/o",
		NewRoot:   "/n",
		CreatedAt: ts,
		Totals:    &Totals{ChangedFiles: 1, SizeBytes: 9},
	}
	entries := []Entry{{Path: "/sorted", OldSize: 0, NewSize: 9, OldStr: "0B", NewStr: "9B"}}
	if err := WriteJSONL(path, fullMeta, entries); err != nil {
		t.Fatal(err)
	}
	gotMeta, gotEntries, err := ParseJSONLFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.Totals == nil || gotMeta.Totals.ChangedFiles != 1 {
		t.Fatalf("final meta: %+v", gotMeta)
	}
	if len(gotEntries) != 1 || gotEntries[0].Path != "/sorted" {
		t.Fatalf("final entries: %#v", gotEntries)
	}
}

// Детерминированные строки размера для снапшотов UI.
func TestFormatDisplay_modesDiffer(t *testing.T) {
	n := int64(1536)
	a := FormatDisplay(n, false, false)
	b := FormatDisplay(n, true, false)
	c := FormatDisplay(n, false, true)
	if a == b {
		t.Fatalf("base2 vs base10 should differ: %q vs %q", a, b)
	}
	if a == "" || b == "" || c == "" {
		t.Fatal("empty format")
	}
}
