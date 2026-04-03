//go:build linux

package savedialog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// PickSaveReportJSONL uses zenity when available (`apt install zenity`).
func PickSaveReportJSONL(suggested string) (string, error) {
	zenity, err := exec.LookPath("zenity")
	if err != nil {
		return "", ErrUnavailable
	}
	name := SanitizeSuggestedName(suggested)
	cmd := exec.Command(zenity, "--file-selection", "--save", "--confirm-overwrite",
		"--filename="+name, "--title=Сохранить отчёт JSONL", "--file-filter=JSONL (*.jsonl) | *.jsonl *.JSONL | *.jsonl.gz *.JSONL.GZ")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		// zenity exit 1 = cancel
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", ErrCanceled
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("zenity: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("zenity: %w", err)
	}
	path := strings.TrimSpace(out.String())
	if path == "" {
		return "", ErrCanceled
	}
	return path, nil
}
