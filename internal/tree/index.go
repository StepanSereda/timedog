package tree

import (
	"sort"
	"strings"

	"timedog/internal/report"
)

// Node is a path segment in the report tree.
type Node struct {
	Name     string           `json:"name"`
	FullPath string           `json:"full_path"`
	IsDir    bool             `json:"is_dir"`
	Entry    *report.Entry    `json:"entry,omitempty"`
	Children map[string]*Node `json:"-"`
}

// ChildDTO for API response.
type ChildDTO struct {
	Path  string        `json:"path"`
	Name  string        `json:"name"`
	IsDir bool          `json:"is_dir"`
	Entry *report.Entry `json:"entry,omitempty"`
}

// BuildTree from flat entries (same semantics as index.html buildTree).
func BuildTree(entries []report.Entry) *Node {
	root := &Node{Name: "", FullPath: "/", Children: map[string]*Node{}}

	for i := range entries {
		e := &entries[i]
		raw := strings.TrimSpace(e.Path)
		if raw == "" || raw == "/" {
			root.Entry = e
			continue
		}
		trim := strings.TrimSuffix(raw, "/")
		parts := strings.Split(strings.Trim(trim, "/"), "/")
		var clean []string
		for _, p := range parts {
			if p != "" {
				clean = append(clean, p)
			}
		}
		if len(clean) == 0 {
			continue
		}
		cur := root
		curPath := ""
		for j, seg := range clean {
			if cur.Children == nil {
				cur.Children = map[string]*Node{}
			}
			curPath = curPath + "/" + seg
			ch, ok := cur.Children[seg]
			if !ok {
				ch = &Node{Name: seg, Children: map[string]*Node{}}
				cur.Children[seg] = ch
			}
			isLast := j == len(clean)-1
			if isLast {
				ch.Entry = e
				ch.IsDir = e.IsDir
				if e.IsDir {
					ch.FullPath = curPath + "/"
				} else {
					ch.FullPath = curPath
				}
			} else {
				ch.IsDir = true
				ch.FullPath = curPath + "/"
			}
			cur = ch
		}
	}
	return root
}

// EntryClass mirrors frontend chip classification for one report row.
func EntryClass(e *report.Entry) string {
	if e == nil {
		return "same"
	}
	if e.UnknownOld {
		return "rem"
	}
	o, n := e.OldSize, e.NewSize
	if o == 0 && n == 0 {
		return "same"
	}
	if o == 0 && n > 0 {
		return "new"
	}
	if o > 0 && n == 0 {
		return "rem"
	}
	if n > o {
		return "inc"
	}
	if n < o {
		return "dec"
	}
	return "same"
}

func filterMatches(class, filter string) bool {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "", "all":
		return true
	case "same":
		return class == "same"
	case "new":
		return class == "new"
	case "removed", "rem":
		return class == "rem"
	case "changed":
		return class == "inc" || class == "dec"
	default:
		return true
	}
}

func subtreeMatchesChip(n *Node, filter string) bool {
	if filter == "" || strings.ToLower(filter) == "all" {
		return true
	}
	if n.Entry != nil && filterMatches(EntryClass(n.Entry), filter) {
		return true
	}
	for _, ch := range n.Children {
		if subtreeMatchesChip(ch, filter) {
			return true
		}
	}
	return false
}

func subtreeMatchesSearch(n *Node, ql string) bool {
	if ql == "" {
		return true
	}
	if strings.Contains(strings.ToLower(n.FullPath), ql) || strings.Contains(strings.ToLower(n.Name), ql) {
		return true
	}
	for _, ch := range n.Children {
		if subtreeMatchesSearch(ch, ql) {
			return true
		}
	}
	return false
}

// FindNode navigates from root by logical full path (with or without trailing slash).
func FindNode(root *Node, fullPath string) *Node {
	fullPath = strings.TrimSpace(fullPath)
	fullPath = strings.TrimSuffix(fullPath, "/")
	if fullPath == "" || fullPath == "/" {
		return root
	}
	p := strings.Trim(fullPath, "/")
	parts := strings.Split(p, "/")
	cur := root
	for _, seg := range parts {
		if seg == "" {
			continue
		}
		if cur.Children == nil {
			return nil
		}
		ch, ok := cur.Children[seg]
		if !ok {
			return nil
		}
		cur = ch
	}
	return cur
}

// ListChildDTOsFiltered applies chip filter and optional substring search on full path / name in subtree.
func ListChildDTOsFiltered(root *Node, prefixPath, filter, search string) []ChildDTO {
	children := ListChildDTOs(root, prefixPath)
	if (filter == "" || strings.ToLower(filter) == "all") && strings.TrimSpace(search) == "" {
		return children
	}
	ql := strings.ToLower(strings.TrimSpace(search))
	out := make([]ChildDTO, 0, len(children))
	for _, ch := range children {
		node := FindNode(root, ch.Path)
		if node == nil {
			continue
		}
		if !subtreeMatchesChip(node, filter) {
			continue
		}
		if !subtreeMatchesSearch(node, ql) {
			continue
		}
		out = append(out, ch)
	}
	return out
}

// ListChildDTOs returns sorted children under prefix ("/" or "/Data/").
func ListChildDTOs(root *Node, prefixPath string) []ChildDTO {
	prefixPath = strings.TrimSpace(prefixPath)
	if prefixPath == "" {
		prefixPath = "/"
	}
	node := root
	if prefixPath != "/" {
		p := strings.Trim(prefixPath, "/")
		parts := strings.Split(p, "/")
		for _, seg := range parts {
			if seg == "" {
				continue
			}
			if node.Children == nil {
				return nil
			}
			ch, ok := node.Children[seg]
			if !ok {
				return nil
			}
			node = ch
		}
	}
	if node.Children == nil {
		return nil
	}
	var keys []string
	for k := range node.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ChildDTO, 0, len(keys))
	for _, k := range keys {
		n := node.Children[k]
		out = append(out, ChildDTO{
			Path:  n.FullPath,
			Name:  n.Name,
			IsDir: n.IsDir || len(n.Children) > 0,
			Entry: entryForListDTO(n),
		})
	}
	return out
}

// subtreeSizeTotals sums OldSize/NewSize of this node’s Entry and all descendants (each report line once).
func subtreeSizeTotals(n *Node) (old, new int64, unknownOld bool) {
	if n.Entry != nil {
		old += n.Entry.OldSize
		new += n.Entry.NewSize
		unknownOld = unknownOld || n.Entry.UnknownOld
	}
	for _, ch := range n.Children {
		o, ne, u := subtreeSizeTotals(ch)
		old += o
		new += ne
		unknownOld = unknownOld || u
	}
	return
}

// dirRollupFromChildren aggregates sizes of all descendant report entries (each child’s full subtree).
func dirRollupFromChildren(n *Node) (old, new int64, unknownOld bool) {
	for _, ch := range n.Children {
		o, ne, u := subtreeSizeTotals(ch)
		old += o
		new += ne
		unknownOld = unknownOld || u
	}
	return
}

// entryForListDTO: для папок с дочерними узлами показываем сумму размеров по содержимому (дочерние поддеревья).
func entryForListDTO(n *Node) *report.Entry {
	hasKids := len(n.Children) > 0
	if !hasKids {
		return n.Entry
	}
	oldSum, newSum, unk := dirRollupFromChildren(n)
	oldStr := report.FormatBytes(oldSum)
	if unk {
		oldStr = "..."
	}
	inDir := 0
	if n.Entry != nil {
		inDir = n.Entry.InDir
	}
	syn := &report.Entry{
		Kind:       report.KindEntry,
		Path:       n.FullPath,
		OldSize:    oldSum,
		NewSize:    newSum,
		OldStr:     oldStr,
		NewStr:     report.FormatBytes(newSum),
		IsDir:      n.IsDir || hasKids,
		UnknownOld: unk,
		InDir:      inDir,
	}
	return syn
}

// Summary counts for chips.
func Summary(entries []report.Entry) (same, changed, nu, rem int) {
	for i := range entries {
		e := &entries[i]
		dc := EntryClass(e)
		switch dc {
		case "same":
			same++
		case "new":
			nu++
		case "rem":
			rem++
		default:
			changed++
		}
	}
	return
}
