package report

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"strings"
)

// StreamReportWriter writes the meta line first, then one JSON line per WriteEntry with flush
// so the file grows during a long scan. Not safe for concurrent WriteEntry — caller must serialize.
// Close must be called before WriteJSONL replaces the same path with the final sorted report.
type StreamReportWriter struct {
	f  *os.File
	gz *gzip.Writer
	bw *bufio.Writer
	enc *json.Encoder
}

// NewStreamReportWriter creates or truncates path, writes meta as the first line, and returns
// a writer for streaming entry lines. meta should omit totals and skipped (filled in final WriteJSONL).
func NewStreamReportWriter(path string, meta Meta) (*StreamReportWriter, error) {
	meta.Kind = KindMeta
	if meta.V == 0 {
		meta.V = 1
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &StreamReportWriter{f: f}
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		w.gz = gzip.NewWriter(f)
		w.bw = bufio.NewWriterSize(w.gz, 256*1024)
	} else {
		w.bw = bufio.NewWriterSize(f, 256*1024)
	}
	w.enc = json.NewEncoder(w.bw)
	w.enc.SetEscapeHTML(false)
	if err := w.enc.Encode(meta); err != nil {
		w.abort()
		return nil, err
	}
	if err := w.bw.Flush(); err != nil {
		w.abort()
		return nil, err
	}
	if w.gz != nil {
		if err := w.gz.Flush(); err != nil {
			w.abort()
			return nil, err
		}
	}
	return w, nil
}

func (w *StreamReportWriter) abort() {
	if w.bw != nil {
		_ = w.bw.Flush()
	}
	if w.gz != nil {
		_ = w.gz.Close()
		w.gz = nil
	}
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	w.bw = nil
	w.enc = nil
}

// WriteEntry appends one entry line and flushes so consumers (e.g. tail -f) see progress.
func (w *StreamReportWriter) WriteEntry(e Entry) error {
	e.Kind = KindEntry
	if err := w.enc.Encode(e); err != nil {
		return err
	}
	if err := w.bw.Flush(); err != nil {
		return err
	}
	if w.gz != nil {
		return w.gz.Flush()
	}
	return nil
}

// Close flushes and closes the underlying file.
func (w *StreamReportWriter) Close() error {
	if w.f == nil {
		return nil
	}
	if err := w.bw.Flush(); err != nil {
		w.abort()
		return err
	}
	if w.gz != nil {
		if err := w.gz.Close(); err != nil {
			w.gz = nil
			if w.f != nil {
				_ = w.f.Close()
				w.f = nil
			}
			w.bw = nil
			return err
		}
		w.gz = nil
	}
	err := w.f.Sync()
	cerr := w.f.Close()
	w.f = nil
	w.bw = nil
	w.enc = nil
	if err != nil {
		return err
	}
	return cerr
}
