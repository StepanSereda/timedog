package tmutil

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Snapshot is one Time Machine backup path from tmutil listbackups -m
type Snapshot struct {
	Path  string `json:"path"`
	Label string `json:"label"` // YYYY-MM-DD-HHMMSS
}

var stampRe = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}-\d{6})`)

// ListBackups runs `tmutil listbackups -m` (macOS).
func ListBackups() ([]Snapshot, error) {
	cmd := exec.Command("tmutil", "listbackups", "-m")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w", errTMutilList(err, out))
	}
	var list []Snapshot
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte{'\n'}) {
		p := strings.TrimSpace(string(line))
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		label := extractLabel(abs)
		list = append(list, Snapshot{Path: abs, Label: label})
	}
	return list, nil
}

func extractLabel(p string) string {
	m := stampRe.FindStringSubmatch(p)
	if len(m) > 1 {
		return m[1]
	}
	return filepath.Base(p)
}

const tmutilHints = ` Подсказки: включена ли Time Machine и есть ли хотя бы один бэкап; смонтирован ли диск бэкапов; ` +
	`для приложения, которое запускает сервер (Terminal.app, iTerm, Cursor и т.д.), в «Системные настройки → Конфиденциальность и безопасность → Полный доступ к диску» ` +
	`должен быть разрешён Full Disk Access; после смены прав перезапустите терминал и сервер. Проверка вручную: tmutil listbackups -m`

func errTMutilList(err error, combinedOut []byte) error {
	msg := strings.TrimSpace(string(combinedOut))
	var exit *exec.ExitError
	code := -1
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "tmutil listbackups: %v", err)
	if msg != "" {
		fmt.Fprintf(&b, " (%s)", msg)
	}
	if code >= 0 {
		fmt.Fprintf(&b, " [код выхода %d]", code)
	}
	b.WriteString(".")
	b.WriteString(tmutilHints)
	return errors.New(b.String())
}
