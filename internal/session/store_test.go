package session

import (
	"testing"

	"timedog/internal/report"
	"timedog/internal/tree"
)

// Гарантии: после импорта отчёта дерево в памяти согласовано с записями; сессия удаляется из реестра.

func TestNewSession_buildsTreeAndGet(t *testing.T) {
	meta := report.Meta{OldRoot: "/o", NewRoot: "/n"}
	entries := []report.Entry{
		{Path: "/A/f", OldSize: 1, NewSize: 2},
	}

	s := NewSession(meta, entries)
	t.Cleanup(func() { Delete(s.ID) })

	got, ok := Get(s.ID)
	if !ok || got.TreeRoot == nil {
		t.Fatal("session not found")
	}
	ch := tree.ListChildDTOs(got.TreeRoot, "/")
	if len(ch) != 1 || ch[0].Name != "A" {
		t.Fatalf("children: %+v", ch)
	}
	if len(got.Entries) != 1 {
		t.Fatal(len(got.Entries))
	}
}

func TestDelete_removesSession(t *testing.T) {
	s := NewSession(report.Meta{}, nil)
	id := s.ID
	Delete(id)
	if _, ok := Get(id); ok {
		t.Fatal("session still present")
	}
}
