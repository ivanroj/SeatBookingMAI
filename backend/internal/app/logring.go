package app

import (
	"sync"
	"time"
)

// LogEntry is a single domain event captured for the admin Журнал viewer.
// Kept lean: timestamp + event name + free-form fields.
type LogEntry struct {
	At     time.Time      `json:"at"`
	Event  string         `json:"event"`
	Fields map[string]any `json:"fields,omitempty"`
}

// logRing is a fixed-size in-memory ring of recent log entries. Writes are
// O(1); reads return up to `limit` newest-first entries. Safe for concurrent
// use. The buffer does not persist across restarts — it is intended only as a
// recent-activity surface for admins, not as audit storage.
type logRing struct {
	mu     sync.Mutex
	buf    []LogEntry
	cap    int
	head   int
	filled bool
}

func newLogRing(capacity int) *logRing {
	if capacity <= 0 {
		capacity = 100
	}
	return &logRing{buf: make([]LogEntry, capacity), cap: capacity}
}

func (r *logRing) Push(entry LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = entry
	r.head = (r.head + 1) % r.cap
	if r.head == 0 {
		r.filled = true
	}
}

// Snapshot returns up to `limit` most-recent entries, newest first.
func (r *logRing) Snapshot(limit int) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.head
	if r.filled {
		size = r.cap
	}
	if limit <= 0 || limit > size {
		limit = size
	}
	out := make([]LogEntry, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (r.head - 1 - i + r.cap) % r.cap
		out = append(out, r.buf[idx])
	}
	return out
}
