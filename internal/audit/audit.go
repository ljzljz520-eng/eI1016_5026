package audit

import (
	"sync"
	"time"
)

type Entry struct {
	Sequence int       `json:"sequence"`
	Time     time.Time `json:"time"`
	Username string    `json:"username"`
	Action   string    `json:"action"`
	Outcome  string    `json:"outcome"`
}

type Log interface {
	Append(username, action, outcome string)
	List() []Entry
}

type MemoryLog struct {
	mu      sync.Mutex
	base    time.Time
	entries []Entry
}

func NewMemoryLog(base time.Time) *MemoryLog {
	return &MemoryLog{base: base}
}

func (l *MemoryLog) Append(username, action, outcome string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	sequence := len(l.entries) + 1
	l.entries = append(l.entries, Entry{
		Sequence: sequence,
		Time:     l.base.Add(time.Duration(sequence-1) * time.Second),
		Username: username,
		Action:   action,
		Outcome:  outcome,
	})
}

func (l *MemoryLog) List() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	entries := make([]Entry, len(l.entries))
	copy(entries, l.entries)
	return entries
}
