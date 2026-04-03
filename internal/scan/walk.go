// Package scan обходит «новый» корень снимка и сравнивает inode/размер со «старым» (как Perl timedog).
//
// # Производительность на медленном внешнем диске (Time Machine по USB и т.п.)
//
// Узкое место — не «чтение файлов», а метаданные: readdir по каталогам и два Lstat на каждый
// затронутый путь (новый снимок + старый). Латентность USB/HDD суммируется по числу узлов.
//
// Уже сделано в коде:
//   - При совпадении inode каталога поддерево не обходится (fs.SkipDir) — главный выигрыш на TM.
//   - Если Lstat на новом пути успешен, DirEntry.Info не вызывается (на многих FS это лишний stat).
//   - Размеры в отчёте с обеих сторон из одного семейства вызовов Lstat.
//
// Параллельный обход: по умолчанию github.com/charlievieth/fastwalk (options.fast_walk, по умолчанию true).
// На одном внешнем HDD можно отключить (fast_walk: false), если seek‑паттерн окажется хуже.
// Фоновый job сразу создаёт файл отчёта и дописывает строку на каждый изменённый путь (report.StreamReportWriter);
// после обхода файл перезаписывается финальным отчётом (сортировка, -d, totals, skipped).
//
// Вне кода: USB 3, кабель/порт, не уводить Mac в сон, при сетевом TM — стабильная сеть; флаги -m/-l
// уменьшают размер отчёта и работу после сортировки, но не ускоряют сам обход.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
	"timedog/internal/report"
)

const maxSkippedStored = 5000

type progressFn func(processedPaths int64)

// Result of scan.Run (изменения + пути, которые не удалось обработать).
type Result struct {
	Entries          []report.Entry
	Skipped          []report.SkippedPath
	SkippedTotal     int
	SkippedTruncated bool
	Totals           report.Totals
}

// relPathUnderRoot maps absolute path under snapshot to logical /… path (как в отчёте).
func relPathUnderRoot(snapshotRoot, abs string) string {
	rel := strings.TrimPrefix(abs, snapshotRoot)
	if rel == abs {
		pref := snapshotRoot + string(filepath.Separator)
		rel = strings.TrimPrefix(abs, pref)
	}
	if rel == "" {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}

// shouldSkipAccessError: при обходе TM часть каталогов недоступна (ParentalControls, sandbox и т.д.).
func shouldSkipAccessError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, os.ErrPermission) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES, syscall.EPERM:
			return true
		}
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "permission denied") ||
		strings.Contains(s, "operation not permitted")
}

type walkState struct {
	mu           sync.Mutex
	entries      []report.Entry
	skipped      []report.SkippedPath
	skippedTotal int
}

func (s *walkState) recordSkip(newRoot, abs string, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skippedTotal++
	if len(s.skipped) < maxSkippedStored {
		s.skipped = append(s.skipped, report.SkippedPath{
			Path:   relPathUnderRoot(newRoot, abs),
			Reason: reason,
		})
	}
}

func (s *walkState) appendEntry(e report.Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
}

// Run walks newRoot like timedog (Perl File::Find + lstat): entries are paths that changed
// (inode mismatch or missing on old side). Matches timedog's silent stat failures: on the new
// side a failed lstat yields falsy $ino, so the path is not pruned and still counted — we must
// not drop those paths entirely (see timedog lines 232–245, summarize after the prune block).
//
// emitEntry is optional: called once per changed path as soon as it is discovered (walk order).
// The caller may stream JSON lines to disk; final report must still be written after Run returns
// (sort, -d rollup, totals, skipped). Safe for concurrent emitEntry from fastwalk if the callback serializes.
func Run(ctx context.Context, oldRoot, newRoot string, opts Options, onProgress progressFn, emitEntry func(report.Entry) error) (Result, error) {
	oldRoot = filepath.Clean(oldRoot)
	newRoot = filepath.Clean(newRoot)
	if oldRoot == newRoot {
		return Result{}, fmt.Errorf("old and new roots must differ")
	}

	var st walkState
	var processed atomic.Int64

	visit := func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// Как timedog: не ронять скан из‑за каталогов без прав (ParentalControls, SIP и т.д.).
			if shouldSkipAccessError(err) {
				st.recordSkip(newRoot, path, "walk")
				return nil
			}
			return err
		}

		n := processed.Add(1)
		if onProgress != nil && n%500 == 0 {
			onProgress(n)
		}

		oldPath := strings.Replace(path, newRoot, oldRoot, 1)
		rel := relPathUnderRoot(newRoot, path)

		// Сначала Lstat(новый): при успехе тип/режим берём из него и не зовём d.Info() (лишний stat на многих FS).
		newIno, newModeU32, newSizeLstat, errN := lstatStat(path)
		var mode os.FileMode
		var fi fs.FileInfo
		var infoErr error
		if errN == nil {
			mode = os.FileMode(newModeU32)
		} else {
			fi, infoErr = d.Info()
			if infoErr != nil {
				if !shouldSkipAccessError(infoErr) {
					return infoErr
				}
				mode = d.Type()
			} else {
				mode = fi.Mode()
			}
		}

		if opts.OmitSymlinks && mode&os.ModeSymlink != 0 {
			return nil
		}

		oldIno, _, oldSizeLstat, errO := lstatStat(oldPath)
		// Оба inode должны быть известны и совпадать, иначе путь не отсекаем (как $ino && $ino_old в Perl).
		sameNode := errN == nil && errO == nil && newIno == oldIno && newIno != 0

		if sameNode {
			if mode.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Размеры только из Lstat — то же семейство вызовов, что и у Perl lstat; не смешивать с d.Info().Size(),
		// иначе на редких типах/кэше DirEntry возможны рассинхрон старого и нового в отчёте при идентичном содержимом.
		var newSize int64
		if errN == nil {
			newSize = newSizeLstat
		} else if infoErr == nil && fi != nil {
			newSize = fi.Size()
		}

		var oldSize int64
		unknownOld := false
		if errO != nil {
			unknownOld = true
			oldSize = 0
		} else {
			oldSize = oldSizeLstat
		}

		if opts.MinSizeBytes != nil && newSize < *opts.MinSizeBytes {
			return nil
		}

		displayPath := rel
		if mode.IsDir() {
			if !strings.HasSuffix(displayPath, "/") && displayPath != "/" {
				displayPath += "/"
			}
		}
		if mode&os.ModeSymlink != 0 {
			displayPath += "@"
		}

		e := report.Entry{
			Kind:       report.KindEntry,
			Path:       displayPath,
			OldSize:    oldSize,
			NewSize:    newSize,
			OldStr:     sizeStr(oldSize, unknownOld, opts),
			NewStr:     report.FormatDisplay(newSize, opts.UseBase10, opts.SimpleFormat),
			IsDir:      mode.IsDir(),
			IsSymlink:  mode&os.ModeSymlink != 0,
			UnknownOld: unknownOld,
		}

		st.appendEntry(e)
		if emitEntry != nil {
			if err := emitEntry(e); err != nil {
				return err
			}
		}
		return nil
	}

	var err error
	if opts.fastWalkEnabled() {
		err = fastwalk.Walk(nil, newRoot, visit)
	} else {
		err = filepath.WalkDir(newRoot, visit)
	}
	if err != nil {
		return Result{}, err
	}

	if onProgress != nil {
		onProgress(processed.Load())
	}

	entries := st.entries
	skipped := st.skipped
	skippedTotal := st.skippedTotal

	sortEntries(entries, opts.SortBy)

	if opts.Depth != nil && *opts.Depth > 0 {
		entries = rollupByDepth(entries, *opts.Depth, opts)
	}

	var sumNew int64
	for i := range entries {
		sumNew += entries[i].NewSize
	}

	sort.Slice(skipped, func(i, j int) bool {
		if skipped[i].Path != skipped[j].Path {
			return skipped[i].Path < skipped[j].Path
		}
		return skipped[i].Reason < skipped[j].Reason
	})

	totals := report.Totals{ChangedFiles: len(entries), SizeBytes: sumNew}
	return Result{
		Entries:          entries,
		Skipped:          skipped,
		SkippedTotal:     skippedTotal,
		SkippedTruncated: skippedTotal > len(skipped),
		Totals:           totals,
	}, nil
}

// RunSequential is the same as Run with fast walk disabled (sequential filepath.WalkDir).
// Intended for tests and reproducible ordering benchmarks.
func RunSequential(ctx context.Context, oldRoot, newRoot string, opts Options, onProgress progressFn) (Result, error) {
	o := opts
	f := false
	o.FastWalk = &f
	return Run(ctx, oldRoot, newRoot, o, onProgress, nil)
}

func lstatStat(path string) (ino uint64, mode uint32, size int64, err error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, 0, 0, err
	}
	return st.Ino, uint32(st.Mode), st.Size, nil
}

func sizeStr(n int64, unknown bool, opts Options) string {
	if unknown {
		return "..."
	}
	if opts.SimpleFormat {
		return report.FormatBytesSimple(n, opts.UseBase10)
	}
	if opts.UseBase10 {
		return report.FormatBytesDecimal(n)
	}
	return report.FormatBytes(n)
}

func sortEntries(entries []report.Entry, sortBy int) {
	switch sortBy {
	case 1:
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].NewSize != entries[j].NewSize {
				return entries[i].NewSize < entries[j].NewSize
			}
			return entries[i].Path < entries[j].Path
		})
	case 2:
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Path < entries[j].Path
		})
	default:
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].OldSize != entries[j].OldSize {
				return entries[i].OldSize < entries[j].OldSize
			}
			return entries[i].Path < entries[j].Path
		})
	}
}
