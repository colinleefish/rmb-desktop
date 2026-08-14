package debug

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const defaultLogCapacity = 2000

// LogEntry is one captured log line.
type LogEntry struct {
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// LogBuffer is a ring buffer slog.Handler for HTTP log tailing.
type LogBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
}

// NewLogBuffer creates a ring buffer with the given capacity.
func NewLogBuffer(capacity int) *LogBuffer {
	if capacity <= 0 {
		capacity = defaultLogCapacity
	}
	return &LogBuffer{capacity: capacity}
}

// Handler returns a slog.Handler that writes into the buffer.
func (b *LogBuffer) Handler(base slog.Handler) slog.Handler {
	if b == nil {
		return base
	}
	return &bufferHandler{buf: b, next: base}
}

func (b *LogBuffer) append(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= b.capacity {
		copy(b.entries, b.entries[1:])
		b.entries[len(b.entries)-1] = entry
		return
	}
	b.entries = append(b.entries, entry)
}

// Tail returns the last n entries, optionally filtered.
func (b *LogBuffer) Tail(n int, level, worker string) []LogEntry {
	if b == nil {
		return []LogEntry{}
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	items := b.entries
	if level != "" {
		minLevel := parseLevel(level)
		filtered := make([]LogEntry, 0, len(items))
		for _, e := range items {
			if levelGE(e.Level, minLevel) {
				filtered = append(filtered, e)
			}
		}
		items = filtered
	}
	if worker != "" {
		filtered := make([]LogEntry, 0, len(items))
		for _, e := range items {
			if matchWorker(e.Attrs, worker) {
				filtered = append(filtered, e)
			}
		}
		items = filtered
	}
	if n <= 0 || n > len(items) {
		n = len(items)
	}
	if n == 0 {
		return []LogEntry{}
	}
	return append([]LogEntry(nil), items[len(items)-n:]...)
}

func matchWorker(attrs map[string]any, worker string) bool {
	if attrs == nil {
		return false
	}
	for _, key := range []string{"worker", "name"} {
		if v, ok := attrs[key]; ok {
			if strings.EqualFold(toString(v), worker) {
				return true
			}
		}
	}
	return false
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type bufferHandler struct {
	buf  *LogBuffer
	next slog.Handler
}

func (h *bufferHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *bufferHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := map[string]any{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = attrToJSON(a)
		return true
	})
	h.buf.append(LogEntry{
		Time:    r.Time.UTC().Format(time.RFC3339Nano),
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   attrs,
	})
	return h.next.Handle(ctx, r)
}

// attrToJSON converts slog attrs to JSON-safe values. Errors become err.Error()
// strings so the debug log API does not emit empty {} objects.
func attrToJSON(a slog.Attr) any {
	if a.Equal(slog.Attr{}) {
		return nil
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return a.Value.String()
	case slog.KindInt64:
		return a.Value.Int64()
	case slog.KindUint64:
		return a.Value.Uint64()
	case slog.KindFloat64:
		return a.Value.Float64()
	case slog.KindBool:
		return a.Value.Bool()
	case slog.KindDuration:
		return a.Value.Duration().String()
	case slog.KindTime:
		return a.Value.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindAny:
		return anyToJSON(a.Value.Any())
	case slog.KindLogValuer:
		return attrToJSON(slog.Any(a.Key, a.Value.LogValuer().LogValue()))
	default:
		return anyToJSON(a.Value.Any())
	}
}

func anyToJSON(v any) any {
	if v == nil {
		return nil
	}
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return v
}

func (h *bufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &bufferHandler{buf: h.buf, next: h.next.WithAttrs(attrs)}
}

func (h *bufferHandler) WithGroup(name string) slog.Handler {
	return &bufferHandler{buf: h.buf, next: h.next.WithGroup(name)}
}

// NewLogger wraps base with an in-memory ring buffer.
func NewLogger(buf *LogBuffer, base slog.Handler) *slog.Logger {
	if buf == nil {
		return slog.New(base)
	}
	return slog.New(buf.Handler(base))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func levelGE(got string, min slog.Level) bool {
	var lv slog.Level
	switch strings.ToLower(got) {
	case "debug":
		lv = slog.LevelDebug
	case "info":
		lv = slog.LevelInfo
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return lv >= min
}
