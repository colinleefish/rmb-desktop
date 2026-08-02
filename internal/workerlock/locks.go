package workerlock

import "sync"

// SessionLocks provides per-session mutual exclusion (replaces PG advisory locks).
type SessionLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewSessionLocks() *SessionLocks {
	return &SessionLocks{locks: make(map[string]*sync.Mutex)}
}

func (s *SessionLocks) Lock(sessionID string) func() {
	s.mu.Lock()
	l, ok := s.locks[sessionID]
	if !ok {
		l = &sync.Mutex{}
		s.locks[sessionID] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// GlobalLock serializes L3 rollup (D6 app-level mutex).
var GlobalLock sync.Mutex
