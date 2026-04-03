package report

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const jsonlFlushEvery = 32 // flush buffered writer + gzip block often enough for tail -f / crash visibility

// WriteJSONL writes meta as first line, then each entry. Uses gzip when path ends with .gz.
// Output is buffered; the buffer is flushed regularly so lines reach the OS soon (streaming to disk).
func WriteJSONL(path string, meta Meta, entries []Entry) error {
	meta.Kind = KindMeta
	if meta.Kind == "" {
		meta.Kind = KindMeta
	}
	if meta.V == 0 {
		meta.V = 1
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	useGzip := strings.HasSuffix(strings.ToLower(path), ".gz")
	if useGzip {
		gz := gzip.NewWriter(f)
		defer func() { _ = gz.Close() }()
		bw := bufio.NewWriterSize(gz, 256*1024)
		enc := json.NewEncoder(bw)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(meta); err != nil {
			return err
		}
		for i := range entries {
			entries[i].Kind = KindEntry
			if err := enc.Encode(entries[i]); err != nil {
				return err
			}
			if (i+1)%jsonlFlushEvery == 0 {
				if err := bw.Flush(); err != nil {
					return err
				}
				if err := gz.Flush(); err != nil {
					return err
				}
			}
		}
		if err := bw.Flush(); err != nil {
			return err
		}
		return gz.Flush()
	}

	bw := bufio.NewWriterSize(f, 256*1024)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(meta); err != nil {
		return err
	}
	for i := range entries {
		entries[i].Kind = KindEntry
		if err := enc.Encode(entries[i]); err != nil {
			return err
		}
		if (i+1)%jsonlFlushEvery == 0 {
			if err := bw.Flush(); err != nil {
				return err
			}
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// ParseJSONL reads first line as Meta, remaining as Entry lines. Detects gzip by magic bytes.
func ParseJSONL(r io.Reader) (Meta, []Entry, error) {
	br := bufio.NewReader(r)
	if head, err := br.Peek(2); err == nil && len(head) == 2 && head[0] == 0x1f && head[1] == 0x8b {
		gr, err := gzip.NewReader(br)
		if err != nil {
			return Meta{}, nil, err
		}
		defer gr.Close()
		r = gr
	} else {
		r = br
	}

	var meta Meta
	var entries []Entry
	sc := bufio.NewScanner(r)
	// allow long lines
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	const maxToken = 32 << 20 // 32 MiB per line
	sc.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		if len(data) >= maxToken {
			return 0, nil, fmt.Errorf("line exceeds %d bytes", maxToken)
		}
		return 0, nil, nil
	})

	lineNum := 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		lineNum++
		if lineNum == 1 {
			if err := json.Unmarshal(line, &meta); err != nil {
				return meta, nil, fmt.Errorf("meta line: %w", err)
			}
			if meta.Kind != KindMeta && meta.Kind != "" {
				// allow kind optional if old files
			}
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return meta, entries, fmt.Errorf("entry line %d: %w", lineNum, err)
		}
		if e.Kind == "summary" {
			continue
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return meta, entries, err
	}
	if lineNum == 0 {
		return meta, nil, fmt.Errorf("empty report")
	}
	return meta, entries, nil
}

// ParseJSONLFile opens path and parses.
func ParseJSONLFile(path string) (Meta, []Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return Meta{}, nil, err
	}
	defer f.Close()
	return ParseJSONL(f)
}

// LabelsFromRoots derives short labels from paths.
func LabelsFromRoots(oldRoot, newRoot string) (oldL, newL string) {
	oldL = extractStamp(oldRoot)
	newL = extractStamp(newRoot)
	return
}

func extractStamp(p string) string {
	p = strings.TrimSuffix(p, "/")
	i := strings.LastIndex(p, "/")
	if i >= 0 {
		return p[i+1:]
	}
	return p
}
