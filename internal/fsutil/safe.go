package fsutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveUnderRoot joins logicalPath (e.g. /Data/foo) with root (snapshot root). Returns absolute path if under root.
func ResolveUnderRoot(root, logicalPath string) (string, error) {
	root = filepath.Clean(root)
	logicalPath = strings.TrimSpace(logicalPath)
	if !strings.HasPrefix(logicalPath, "/") {
		logicalPath = "/" + logicalPath
	}
	rel := strings.TrimPrefix(logicalPath, "/")
	rel = filepath.FromSlash(rel)
	abs := filepath.Join(root, rel)
	abs = filepath.Clean(abs)
	if !isSubpath(abs, root) {
		return "", fmt.Errorf("path escapes root")
	}
	return abs, nil
}

func isSubpath(abs, root string) bool {
	abs = filepath.Clean(abs)
	root = filepath.Clean(root)
	if abs == root {
		return true
	}
	sep := string(filepath.Separator)
	if root == sep {
		return strings.HasPrefix(abs, sep)
	}
	rootPref := root
	if !strings.HasSuffix(rootPref, sep) {
		rootPref = root + sep
	}
	return strings.HasPrefix(abs+sep, rootPref)
}
