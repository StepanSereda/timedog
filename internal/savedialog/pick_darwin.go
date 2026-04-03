//go:build darwin

package savedialog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// PickSaveReportJSONL shows macOS save panel via osascript (GUI session required).
func PickSaveReportJSONL(suggested string) (string, error) {
	name := SanitizeSuggestedName(suggested)
	// Однострочный AppleScript; имя уже безопасное (без кавычек и пробелов в краях).
	script := fmt.Sprintf(
		`POSIX path of (choose file name default name %q with prompt "Сохранить отчёт JSONL (timedog-server)")`,
		name,
	)
	cmd := exec.Command("osascript", "-e", script)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		sErr := strings.ToLower(stderr.String())
		if strings.Contains(sErr, "user canceled") || strings.Contains(sErr, "-128") {
			return "", ErrCanceled
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && out.Len() == 0 {
			return "", ErrCanceled
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("osascript: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("osascript: %w", err)
	}
	path := strings.TrimSpace(out.String())
	if path == "" {
		return "", ErrCanceled
	}
	return path, nil
}
