package jobs

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"timedog/internal/report"
	"timedog/internal/scan"
	"timedog/internal/session"
)

type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusError   Status = "error"
	StatusCanceled Status = "canceled"
)

type ScanJob struct {
	ID                 string
	Status             Status
	Error              string
	Progress           int64
	OutputPath         string
	SessionID          string
	OldRoot            string
	NewRoot            string
	SkippedTotal       int
	SkippedTruncated   bool
	CreatedAt          time.Time
	FinishedAt         *time.Time
	cancel             context.CancelFunc
	mu                 sync.Mutex
	subscribers        []chan Event
}

type Event struct {
	Type               string `json:"type"` // progress|done|error
	Progress           int64  `json:"progress,omitempty"`
	Message            string `json:"message,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	SkippedTotal       int    `json:"skipped_total,omitempty"`
	SkippedTruncated   bool   `json:"skipped_truncated,omitempty"`
}

var (
	jmu  sync.RWMutex
	jobs = map[string]*ScanJob{}
)

func NewScanJob(id string) *ScanJob {
	j := &ScanJob{
		ID:        id,
		Status:    StatusQueued,
		CreatedAt: time.Now(),
	}
	jmu.Lock()
	jobs[id] = j
	jmu.Unlock()
	return j
}

func Get(id string) (*ScanJob, bool) {
	jmu.RLock()
	defer jmu.RUnlock()
	j, ok := jobs[id]
	return j, ok
}

func (j *ScanJob) subscribe() chan Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	ch := make(chan Event, 16)
	j.subscribers = append(j.subscribers, ch)
	return ch
}

func (j *ScanJob) broadcast(ev Event) {
	j.mu.Lock()
	subs := make([]chan Event, len(j.subscribers))
	copy(subs, j.subscribers)
	if ev.Type == "done" || ev.Type == "error" {
		j.subscribers = nil
	}
	j.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
	if ev.Type == "done" || ev.Type == "error" {
		for _, ch := range subs {
			close(ch)
		}
	}
}

// RunScan executes scan and writes JSONL; updates job state.
func (j *ScanJob) RunScan(ctx context.Context, oldRoot, newRoot, outPath string, opts scan.Options) {
	ctx, cancel := context.WithCancel(ctx)
	j.cancel = cancel
	j.Status = StatusRunning
	j.OldRoot = oldRoot
	j.NewRoot = newRoot
	j.OutputPath = outPath

	optsMap := optionsToMap(opts)
	oldLbl, newLbl := report.LabelsFromRoots(oldRoot, newRoot)
	createdAt := time.Now()

	partialMeta := report.Meta{
		V:         1,
		OldRoot:   oldRoot,
		NewRoot:   newRoot,
		OldLabel:  oldLbl,
		NewLabel:  newLbl,
		CreatedAt: createdAt,
		Options:   optsMap,
	}
	stream, err := report.NewStreamReportWriter(outPath, partialMeta)
	if err != nil {
		now := time.Now()
		j.FinishedAt = &now
		j.Status = StatusError
		j.Error = err.Error()
		j.broadcast(Event{Type: "error", Message: j.Error})
		return
	}

	var emitMu sync.Mutex
	emitEntry := func(e report.Entry) error {
		emitMu.Lock()
		defer emitMu.Unlock()
		return stream.WriteEntry(e)
	}

	result, err := scan.Run(ctx, oldRoot, newRoot, opts, func(n int64) {
		j.Progress = n
		j.broadcast(Event{Type: "progress", Progress: n})
	}, emitEntry)
	if err != nil {
		_ = stream.Close()
		now := time.Now()
		j.FinishedAt = &now
		if ctx.Err() != nil {
			j.Status = StatusCanceled
			j.Error = ctx.Err().Error()
		} else {
			j.Status = StatusError
			j.Error = err.Error()
		}
		j.broadcast(Event{Type: "error", Message: j.Error})
		return
	}

	if err := stream.Close(); err != nil {
		now := time.Now()
		j.FinishedAt = &now
		j.Status = StatusError
		j.Error = err.Error()
		j.broadcast(Event{Type: "error", Message: j.Error})
		return
	}

	meta := report.Meta{
		V:                1,
		OldRoot:          oldRoot,
		NewRoot:          newRoot,
		OldLabel:         oldLbl,
		NewLabel:         newLbl,
		CreatedAt:        createdAt,
		Options:          optsMap,
		Totals:           &result.Totals,
		Skipped:          result.Skipped,
		SkippedTotal:     result.SkippedTotal,
		SkippedTruncated: result.SkippedTruncated,
	}
	if err := report.WriteJSONL(outPath, meta, result.Entries); err != nil {
		now := time.Now()
		j.FinishedAt = &now
		j.Status = StatusError
		j.Error = err.Error()
		j.broadcast(Event{Type: "error", Message: j.Error})
		return
	}

	sess := session.NewSession(meta, result.Entries)
	j.SessionID = sess.ID
	j.SkippedTotal = result.SkippedTotal
	j.SkippedTruncated = result.SkippedTruncated
	j.Status = StatusDone
	j.Progress = int64(len(result.Entries))
	now := time.Now()
	j.FinishedAt = &now
	j.broadcast(Event{
		Type: "done", SessionID: sess.ID, Progress: j.Progress,
		SkippedTotal: result.SkippedTotal, SkippedTruncated: result.SkippedTruncated,
	})
}

func (j *ScanJob) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

func (j *ScanJob) SSESubscribe() <-chan Event {
	return j.subscribe()
}

func optionsToMap(o scan.Options) map[string]any {
	m := map[string]any{
		"omit_symlinks": o.OmitSymlinks,
		"sort_by":       o.SortBy,
		"use_base10":    o.UseBase10,
		"simple_format": o.SimpleFormat,
	}
	if o.FastWalk != nil {
		m["fast_walk"] = *o.FastWalk
	}
	if o.Depth != nil {
		m["depth"] = *o.Depth
	}
	if o.MinSizeBytes != nil {
		m["min_size_bytes"] = *o.MinSizeBytes
	}
	return m
}

// Snapshot for API
func (j *ScanJob) SnapshotJSON() ([]byte, error) {
	type snap struct {
		ID                 string `json:"id"`
		Status             Status `json:"status"`
		Error              string `json:"error,omitempty"`
		Progress           int64  `json:"progress"`
		OutputPath         string `json:"output_path,omitempty"`
		SessionID          string `json:"session_id,omitempty"`
		OldRoot            string `json:"old_root,omitempty"`
		NewRoot            string `json:"new_root,omitempty"`
		SkippedTotal       int    `json:"skipped_total,omitempty"`
		SkippedTruncated   bool   `json:"skipped_truncated,omitempty"`
	}
	return json.Marshal(snap{
		ID:               j.ID,
		Status:           j.Status,
		Error:            j.Error,
		Progress:         j.Progress,
		OutputPath:       j.OutputPath,
		SessionID:        j.SessionID,
		OldRoot:          j.OldRoot,
		NewRoot:          j.NewRoot,
		SkippedTotal:     j.SkippedTotal,
		SkippedTruncated: j.SkippedTruncated,
	})
}
