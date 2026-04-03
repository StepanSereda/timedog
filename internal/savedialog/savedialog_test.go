package savedialog

import "testing"

func TestSanitizeSuggestedName(t *testing.T) {
	if g := SanitizeSuggestedName(""); g != "timedog-report.jsonl" {
		t.Fatal(g)
	}
	if g := SanitizeSuggestedName("/tmp/foo.jsonl"); g != "foo.jsonl" {
		t.Fatal(g)
	}
	if g := SanitizeSuggestedName("b.jsonl.gz"); g != "b.jsonl.gz" {
		t.Fatal(g)
	}
	if g := SanitizeSuggestedName("x"); g != "x.jsonl" {
		t.Fatal(g)
	}
}
