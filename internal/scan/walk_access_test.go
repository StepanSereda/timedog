package scan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"
)

func TestShouldSkipAccessError(t *testing.T) {
	if !shouldSkipAccessError(os.ErrPermission) {
		t.Fatal("os.ErrPermission")
	}
	if !shouldSkipAccessError(fs.ErrPermission) {
		t.Fatal("fs.ErrPermission")
	}
	if !shouldSkipAccessError(syscall.EACCES) {
		t.Fatal("EACCES")
	}
	if !shouldSkipAccessError(fmt.Errorf("open /x: %w", syscall.EPERM)) {
		t.Fatal("wrapped EPERM")
	}
	if !shouldSkipAccessError(errors.New("permission denied on path")) {
		t.Fatal("string permission denied")
	}
	if shouldSkipAccessError(nil) {
		t.Fatal("nil")
	}
	if shouldSkipAccessError(errors.New("no such file")) {
		t.Fatal("should not skip ENOENT-style unknown")
	}
}
