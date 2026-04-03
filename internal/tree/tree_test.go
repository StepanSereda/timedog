package tree

import (
	"sort"
	"testing"

	"timedog/internal/report"
)

// Гарантии: плоский отчёт превращается в ожидаемую иерархию; фильтры дерева не «протекают» между режимами.

func TestBuildTree_nestedPaths(t *testing.T) {
	entries := []report.Entry{
		{Path: "/Apps/a.txt", OldSize: 1, NewSize: 2},
		{Path: "/Apps/sub/b.txt", OldSize: 0, NewSize: 10},
		{Path: "/Z.txt", OldSize: 1, NewSize: 1},
	}
	root := BuildTree(entries)

	children := ListChildDTOs(root, "/")
	if len(children) != 2 {
		t.Fatalf("root children: %+v", children)
	}
	if children[0].Name != "Apps" || children[1].Name != "Z.txt" {
		t.Fatalf("sort order: %+v", children)
	}

	underApps := ListChildDTOs(root, "/Apps/")
	if len(underApps) != 2 {
		t.Fatalf("/Apps children: %+v", underApps)
	}
	names := []string{underApps[0].Name, underApps[1].Name}
	sort.Strings(names)
	if names[0] != "a.txt" || names[1] != "sub" {
		t.Fatalf("got %v", names)
	}
}

func TestEntryClass_contract(t *testing.T) {
	tests := []struct {
		name string
		e    report.Entry
		want string
	}{
		{"nil", report.Entry{}, "same"},
		{"new", report.Entry{OldSize: 0, NewSize: 5}, "new"},
		{"removed", report.Entry{OldSize: 3, NewSize: 0}, "rem"},
		{"inc", report.Entry{OldSize: 1, NewSize: 9}, "inc"},
		{"dec", report.Entry{OldSize: 9, NewSize: 1}, "dec"},
		{"unknown_old", report.Entry{UnknownOld: true, OldSize: 0, NewSize: 5}, "rem"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g := EntryClass(&tt.e); g != tt.want {
				t.Fatalf("got %q want %q", g, tt.want)
			}
		})
	}
}

func TestSummary_matchesEntryClasses(t *testing.T) {
	entries := []report.Entry{
		{OldSize: 0, NewSize: 1},
		{OldSize: 1, NewSize: 0},
		{OldSize: 1, NewSize: 2},
		{OldSize: 2, NewSize: 1},
		{OldSize: 0, NewSize: 0},
	}
	same, changed, nu, rem := Summary(entries)
	if nu != 1 || rem != 1 || changed != 2 || same != 1 {
		t.Fatalf("same=%d changed=%d new=%d rem=%d", same, changed, nu, rem)
	}
}

func TestListChildDTOsFiltered_newChip(t *testing.T) {
	entries := []report.Entry{
		{Path: "/Apps/newfile", OldSize: 0, NewSize: 100},
		{Path: "/Apps/other", OldSize: 10, NewSize: 5},
	}
	root := BuildTree(entries)

	all := ListChildDTOsFiltered(root, "/Apps/", "all", "")
	if len(all) != 2 {
		t.Fatalf("all: %+v", all)
	}

	onlyNew := ListChildDTOsFiltered(root, "/Apps/", "new", "")
	if len(onlyNew) != 1 || onlyNew[0].Name != "newfile" {
		t.Fatalf("new filter: %+v", onlyNew)
	}
}

func TestListChildDTOsFiltered_searchSubtree(t *testing.T) {
	entries := []report.Entry{
		{Path: "/foo/bar/baz.txt", OldSize: 1, NewSize: 2},
		{Path: "/foo/qux.txt", OldSize: 1, NewSize: 1},
	}
	root := BuildTree(entries)

	atRoot := ListChildDTOsFiltered(root, "/", "all", "baz")
	if len(atRoot) != 1 || atRoot[0].Name != "foo" {
		t.Fatalf("expected single folder foo containing baz: %+v", atRoot)
	}

	underFoo := ListChildDTOsFiltered(root, "/foo/", "all", "baz")
	if len(underFoo) != 1 || underFoo[0].Name != "bar" {
		t.Fatalf("under /foo/: %+v", underFoo)
	}
}

func TestListChildDTOs_folderRollupSumsChildren(t *testing.T) {
	entries := []report.Entry{
		{Path: "/Apps/a.txt", OldSize: 10, NewSize: 100},
		{Path: "/Apps/b.txt", OldSize: 5, NewSize: 5},
		{Path: "/Z.txt", OldSize: 1, NewSize: 2},
	}
	root := BuildTree(entries)
	children := ListChildDTOs(root, "/")
	if len(children) != 2 || children[0].Name != "Apps" {
		t.Fatalf("root: %+v", children)
	}
	apps := children[0]
	if apps.Entry == nil {
		t.Fatal("rollup entry")
	}
	if apps.Entry.OldSize != 15 || apps.Entry.NewSize != 105 {
		t.Fatalf("Apps rollup sizes: old=%d new=%d", apps.Entry.OldSize, apps.Entry.NewSize)
	}
}

func TestFindNode_roundTrip(t *testing.T) {
	entries := []report.Entry{{Path: "/a/b/c", OldSize: 1, NewSize: 1}}
	root := BuildTree(entries)
	n := FindNode(root, "/a/b/c")
	if n == nil || n.Name != "c" {
		t.Fatal(n)
	}
	if FindNode(root, "/nope") != nil {
		t.Fatal("expected nil")
	}
}
