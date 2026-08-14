package debug

import (
	"os"
	"sync"
	"time"
)

// Registry tracks worker health and in-flight session work for HTTP debug endpoints.
type Registry struct {
	mu         sync.RWMutex
	startedAt  time.Time
	pid        int
	workers    map[string]*workerState
	inFlight   map[string]*InFlightSession
	backpressure map[string]backpressureSnap
}

type workerState struct {
	Name              string     `json:"name"`
	Alive             bool       `json:"alive"`
	LastCycleAt       *time.Time `json:"last_cycle_at,omitempty"`
	LastCycleMS       int64      `json:"last_cycle_duration_ms,omitempty"`
	InFlight          int        `json:"in_flight"`
	LastError         string     `json:"last_error,omitempty"`
	Concurrency       int        `json:"concurrency,omitempty"`
	BlockedSince      *time.Time `json:"blocked_since,omitempty"`
	CycleCount        int64      `json:"cycle_count"`
}

// InFlightSession is one session job currently running inside a worker.
type InFlightSession struct {
	SessionID  string    `json:"session_id"`
	SessionKey string    `json:"session_key,omitempty"`
	Worker     string    `json:"worker"`
	Step       string    `json:"step"`
	StartedAt  time.Time `json:"started_at"`
}

type backpressureSnap struct {
	Min         int `json:"min"`
	Max         int `json:"max"`
	Current     int `json:"current"`
	PendingHint int `json:"pending_hint,omitempty"`
}

// ProcessInfo is process-level metadata.
type ProcessInfo struct {
	StartedAt string `json:"started_at"`
	PID       int    `json:"pid"`
	UptimeSec int64  `json:"uptime_sec"`
}

// WorkersResponse is returned by GET /api/v1/debug/workers.
type WorkersResponse struct {
	Workers   map[string]workerState `json:"workers"`
	Process   ProcessInfo            `json:"process"`
	InFlight  []InFlightSession      `json:"in_flight"`
}

// NewRegistry creates a debug registry. Pass nil from tests to disable instrumentation.
func NewRegistry() *Registry {
	return &Registry{
		startedAt:    time.Now().UTC(),
		pid:          os.Getpid(),
		workers:      make(map[string]*workerState),
		inFlight:     make(map[string]*InFlightSession),
		backpressure: make(map[string]backpressureSnap),
	}
}

func (r *Registry) ensureWorker(name string) *workerState {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workers[name]
	if !ok {
		ws = &workerState{Name: name}
		r.workers[name] = ws
	}
	return ws
}

// WorkerStarted marks a worker goroutine as alive (call at Run() entry).
func (r *Registry) WorkerStarted(name string) {
	if r == nil {
		return
	}
	ws := r.ensureWorker(name)
	if ws == nil {
		return
	}
	r.mu.Lock()
	ws.Alive = true
	r.mu.Unlock()
}

// WorkerStopped marks a worker as no longer running.
func (r *Registry) WorkerStopped(name string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if ws, ok := r.workers[name]; ok {
		ws.Alive = false
	}
	r.mu.Unlock()
}

// BeginCycle records the start of a worker poll cycle.
func (r *Registry) BeginCycle(name string) func(err error) {
	if r == nil {
		return func(error) {}
	}
	start := time.Now().UTC()
	r.mu.Lock()
	ws := r.workers[name]
	if ws == nil {
		ws = &workerState{Name: name}
		r.workers[name] = ws
	}
	ws.BlockedSince = &start
	r.mu.Unlock()

	return func(err error) {
		end := time.Now().UTC()
		dur := end.Sub(start)
		r.mu.Lock()
		defer r.mu.Unlock()
		if ws, ok := r.workers[name]; ok {
			ws.LastCycleAt = &end
			ws.LastCycleMS = dur.Milliseconds()
			ws.BlockedSince = nil
			ws.CycleCount++
			if err != nil {
				ws.LastError = err.Error()
			}
		}
	}
}

// SetConcurrency updates the current concurrency limit for a worker.
func (r *Registry) SetConcurrency(name string, n int) {
	ws := r.ensureWorker(name)
	if ws == nil {
		return
	}
	r.mu.Lock()
	ws.Concurrency = n
	r.mu.Unlock()
}

// SetBackpressure records backpressure controller state.
func (r *Registry) SetBackpressure(name string, min, max, current, pending int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.backpressure[name] = backpressureSnap{
		Min: min, Max: max, Current: current, PendingHint: pending,
	}
	r.mu.Unlock()
}

// BeginSession marks a session as in-flight for a worker step.
func (r *Registry) BeginSession(sessionID, sessionKey, worker, step string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inFlight[sessionID] = &InFlightSession{
		SessionID:  sessionID,
		SessionKey: sessionKey,
		Worker:     worker,
		Step:       step,
		StartedAt:  time.Now().UTC(),
	}
	if ws, ok := r.workers[worker]; ok {
		ws.InFlight = r.countInFlightLocked(worker)
	}
}

// EndSession clears in-flight tracking for a session.
func (r *Registry) EndSession(sessionID, worker string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inFlight, sessionID)
	if ws, ok := r.workers[worker]; ok {
		ws.InFlight = r.countInFlightLocked(worker)
	}
}

func (r *Registry) countInFlightLocked(worker string) int {
	n := 0
	for _, item := range r.inFlight {
		if item.Worker == worker {
			n++
		}
	}
	return n
}

// Snapshot returns a point-in-time view for HTTP handlers.
func (r *Registry) Snapshot() WorkersResponse {
	if r == nil {
		return WorkersResponse{Workers: map[string]workerState{}}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	workers := make(map[string]workerState, len(r.workers))
	for k, v := range r.workers {
		workers[k] = *v
	}
	inFlight := make([]InFlightSession, 0, len(r.inFlight))
	for _, v := range r.inFlight {
		inFlight = append(inFlight, *v)
	}
	uptime := int64(time.Since(r.startedAt).Seconds())
	return WorkersResponse{
		Workers:  workers,
		InFlight: inFlight,
		Process: ProcessInfo{
			StartedAt: r.startedAt.Format(time.RFC3339),
			PID:       r.pid,
			UptimeSec: uptime,
		},
	}
}

// BackpressureResponse is returned by GET /api/v1/debug/backpressure.
type BackpressureResponse struct {
	Workers map[string]backpressureSnap `json:"workers"`
}

// BackpressureSnapshot returns backpressure state.
func (r *Registry) BackpressureSnapshot() BackpressureResponse {
	if r == nil {
		return BackpressureResponse{Workers: map[string]backpressureSnap{}}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]backpressureSnap, len(r.backpressure))
	for k, v := range r.backpressure {
		out[k] = v
	}
	return BackpressureResponse{Workers: out}
}
