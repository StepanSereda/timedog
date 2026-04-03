package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"timedog/internal/report"
	"timedog/internal/tree"
)

// Session binds report meta + in-memory entries for tree API and content resolver.
type Session struct {
	ID        string
	Meta      report.Meta
	Entries   []report.Entry
	TreeRoot  *tree.Node
	CreatedAt time.Time
}

var (
	mu       sync.RWMutex
	sessions = map[string]*Session{}
)

func NewSession(meta report.Meta, entries []report.Entry) *Session {
	id := randomID()
	s := &Session{
		ID:        id,
		Meta:      meta,
		Entries:   entries,
		TreeRoot:  tree.BuildTree(entries),
		CreatedAt: time.Now(),
	}
	mu.Lock()
	sessions[id] = s
	mu.Unlock()
	return s
}

func Get(id string) (*Session, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := sessions[id]
	return s, ok
}

func Delete(id string) {
	mu.Lock()
	delete(sessions, id)
	mu.Unlock()
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
