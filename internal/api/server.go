package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"timedog/internal/fsutil"
	"timedog/internal/jobs"
	"timedog/internal/report"
	"timedog/internal/savedialog"
	"timedog/internal/scan"
	"timedog/internal/session"
	"timedog/internal/tmutil"
	"timedog/internal/tree"
)

// Server holds API mux only (static files wired in main).
type Server struct {
	Mux *http.ServeMux
}

// NewAPIRouter registers only /api/* routes.
func NewAPIRouter() *http.ServeMux {
	mux := http.NewServeMux()
	// Mounted at /api/ with StripPrefix — patterns are without /api
	mux.HandleFunc("GET /snapshots", cors(snapshotsHandler))
	mux.HandleFunc("POST /browse-output-path", cors(browseOutputPathHandler))
	mux.HandleFunc("POST /scan", cors(scanStartHandler))
	mux.HandleFunc("GET /scan/{id}", cors(scanStatusHandler))
	mux.HandleFunc("GET /scan/{id}/events", cors(scanEventsHandler))
	mux.HandleFunc("POST /scan/{id}/cancel", cors(scanCancelHandler))
	mux.HandleFunc("POST /reports/parse", cors(parseReportHandler))
	mux.HandleFunc("DELETE /session/{id}", cors(deleteSessionHandler))
	mux.HandleFunc("GET /session/{id}/meta", cors(sessionMetaHandler))
	mux.HandleFunc("GET /session/{id}/summary", cors(sessionSummaryHandler))
	mux.HandleFunc("GET /session/{id}/tree", cors(sessionTreeHandler))
	mux.HandleFunc("GET /session/{id}/content", cors(sessionContentHandler))
	return mux
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func snapshotsHandler(w http.ResponseWriter, r *http.Request) {
	list, err := tmutil.ListBackups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]string, 0, len(list))
	for _, s := range list {
		out = append(out, map[string]string{"path": s.Path, "label": s.Label})
	}
	writeJSON(w, out)
}

// browseOutputPathHandler opens a native save dialog on the server machine (macOS/Linux zenity).
// Body: {"suggested":"report.jsonl"}. 200 {"path":"..."}, 204 if canceled, 501 if unavailable.
func browseOutputPathHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Suggested string `json:"suggested"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := savedialog.PickSaveReportJSONL(req.Suggested)
	if err != nil {
		if errors.Is(err, savedialog.ErrCanceled) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if errors.Is(err, savedialog.ErrUnavailable) {
			http.Error(w, "На этой системе нет встроенного диалога сохранения. Укажите путь вручную.", http.StatusNotImplemented)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"path": path})
}

type scanRequest struct {
	OldRoot    string       `json:"old_root"`
	NewRoot    string       `json:"new_root"`
	OutputPath string       `json:"output_path"`
	Options    scan.Options `json:"options"`
}

func scanStartHandler(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.OldRoot == "" || req.NewRoot == "" || req.OutputPath == "" {
		http.Error(w, "old_root, new_root, output_path required", http.StatusBadRequest)
		return
	}
	req.OutputPath = filepath.Clean(req.OutputPath)
	dir := filepath.Dir(req.OutputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := randomID()
	job := jobs.NewScanJob(id)
	go job.RunScan(context.Background(), req.OldRoot, req.NewRoot, req.OutputPath, req.Options)
	writeJSON(w, map[string]string{"id": id})
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func scanStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := jobs.Get(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := j.SnapshotJSON()
	_, _ = w.Write(b)
}

func scanEventsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := jobs.Get(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", http.StatusInternalServerError)
		return
	}
	ch := j.SSESubscribe()
	enc := json.NewEncoder(w)
	for ev := range ch {
		fmt.Fprintf(w, "data: ")
		_ = enc.Encode(ev)
		fmt.Fprint(w, "\n")
		fl.Flush()
		if ev.Type == "done" || ev.Type == "error" {
			break
		}
	}
}

func scanCancelHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := jobs.Get(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	j.Cancel()
	w.WriteHeader(http.StatusNoContent)
}

func parseReportHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field required", http.StatusBadRequest)
		return
	}
	defer f.Close()
	meta, entries, err := report.ParseJSONL(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	meta.Kind = report.KindMeta
	sess := session.NewSession(meta, entries)
	writeJSON(w, map[string]any{
		"session_id":  sess.ID,
		"meta":        meta,
		"entry_count": len(entries),
	})
}

func deleteSessionHandler(w http.ResponseWriter, r *http.Request) {
	session.Delete(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func sessionMetaHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := session.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, s.Meta)
}

func sessionSummaryHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := session.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	same, changed, nu, rem := tree.Summary(s.Entries)
	writeJSON(w, map[string]int{
		"same": same, "changed": changed, "new": nu, "removed": rem,
		"total": len(s.Entries),
	})
}

func sessionTreeHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := session.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}
	filter := r.URL.Query().Get("filter")
	search := r.URL.Query().Get("q")
	children := tree.ListChildDTOsFiltered(s.TreeRoot, prefix, filter, search)
	writeJSON(w, map[string]any{"prefix": prefix, "children": children})
}

func normalizeLogicalPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "@")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func sessionContentHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := session.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	logical := normalizeLogicalPath(r.URL.Query().Get("path"))
	if logical == "" || logical == "/" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "text"
	}
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if offset < 0 {
		offset = 0
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 4*1024*1024 {
		limit = 262144
	}
	side := r.URL.Query().Get("side")
	if side == "" {
		side = "both"
	}
	oldRoot := s.Meta.OldRoot
	newRoot := s.Meta.NewRoot
	if oldRoot == "" || newRoot == "" {
		http.Error(w, "session missing snapshot roots", http.StatusBadRequest)
		return
	}
	resp := map[string]any{}
	if side == "old" || side == "both" {
		ap, err := fsutil.ResolveUnderRoot(oldRoot, logical)
		if err != nil {
			resp["old"] = map[string]any{"error": err.Error()}
		} else {
			resp["old"] = readSlice(ap, mode, offset, limit)
		}
	}
	if side == "new" || side == "both" {
		ap, err := fsutil.ResolveUnderRoot(newRoot, logical)
		if err != nil {
			resp["new"] = map[string]any{"error": err.Error()}
		} else {
			resp["new"] = readSlice(ap, mode, offset, limit)
		}
	}
	writeJSON(w, resp)
}

func readSlice(abs string, mode string, offset int64, limit int) map[string]any {
	fi, err := os.Stat(abs)
	if err != nil {
		return map[string]any{"exists": false, "error": err.Error()}
	}
	if fi.IsDir() {
		return map[string]any{"exists": true, "is_dir": true, "size": fi.Size()}
	}
	f, err := os.Open(abs)
	if err != nil {
		return map[string]any{"exists": false, "error": err.Error()}
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return map[string]any{"error": err.Error()}
	}
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return map[string]any{"error": err.Error()}
	}
	buf = buf[:n]
	out := map[string]any{
		"exists":  true,
		"size":    fi.Size(),
		"offset":  offset,
		"read":    len(buf),
		"raw_b64": base64.StdEncoding.EncodeToString(buf),
	}
	if mode == "hex" {
		out["hex"] = toHexDump(buf, offset)
	} else {
		out["text"] = safeUTF8(buf)
		out["has_more"] = offset+int64(len(buf)) < fi.Size()
	}
	return out
}

func safeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

func toHexDump(b []byte, baseOffset int64) string {
	var sb strings.Builder
	for i := 0; i < len(b); i += 16 {
		end := i + 16
		if end > len(b) {
			end = len(b)
		}
		chunk := b[i:end]
		sb.WriteString(fmt.Sprintf("%08x  ", baseOffset+int64(i)))
		for j := 0; j < len(chunk); j++ {
			sb.WriteString(fmt.Sprintf("%02x ", chunk[j]))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
