package memory

import "sync"

// rollupBatch tracks memory URIs already written during one rollup so a
// variant merge and its incumbent bucket cannot both persist in the same pass.
type rollupBatch struct {
	mu      sync.Mutex
	written map[string]struct{}
}

func newRollupBatch() *rollupBatch {
	return &rollupBatch{written: make(map[string]struct{})}
}

func (b *rollupBatch) alreadyWritten(uri string) bool {
	if b == nil || uri == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.written[uri]
	return ok
}

func (b *rollupBatch) markWritten(uri string) {
	if b == nil || uri == "" {
		return
	}
	b.mu.Lock()
	b.written[uri] = struct{}{}
	b.mu.Unlock()
}

// memoryURILocks serializes materiality + persist for one active memory URI
// across concurrent bucket goroutines (B05 / issue #72).
var memoryURILocks sync.Map // uri -> *sync.Mutex

func lockMemoryURI(uri string) func() {
	if rollupTestHook != nil && rollupTestHook.BypassURILock {
		return func() {}
	}
	if uri == "" {
		return func() {}
	}
	m, _ := memoryURILocks.LoadOrStore(uri, &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	return func() { m.(*sync.Mutex).Unlock() }
}
