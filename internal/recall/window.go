package recall

import (
	"fmt"
	"strings"
	"time"
)

// TimeWindow bounds search results by row timestamp (updated_at, ms epoch).
// A zero value applies no filtering. SinceMS/UntilMS of 0 mean unbounded.
type TimeWindow struct {
	SinceMS int64
	UntilMS int64
}

// Clause returns a SQL fragment (starting with a space, e.g. " AND col >= ?")
// plus args for embedding into a WHERE clause. Empty when unbounded.
func (tw TimeWindow) Clause(col string) (string, []any) {
	var sb strings.Builder
	var args []any
	if tw.SinceMS > 0 {
		sb.WriteString(fmt.Sprintf(" AND %s >= ?", col))
		args = append(args, tw.SinceMS)
	}
	if tw.UntilMS > 0 {
		sb.WriteString(fmt.Sprintf(" AND %s <= ?", col))
		args = append(args, tw.UntilMS)
	}
	return sb.String(), args
}

// ParseTimeValue parses a --since/--until value into a ms-epoch timestamp.
//
// Accepted forms:
//   - absolute: 2026-08-01, 2026-08-01T15:04, 2026-08-01T15:04:05
//     (date-only values resolve to local midnight; timestamps without a
//     zone resolve in the local zone)
//   - relative: 30m, 12h, 7d (ago, relative to now)
func ParseTimeValue(raw string, now time.Time) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("empty time value")
	}

	// Relative durations: Nd / Nh / Nm.
	if n := len(s); n > 1 {
		unit := s[n-1]
		numPart := s[:n-1]
		var unitDur time.Duration
		switch unit {
		case 'd':
			unitDur = 24 * time.Hour
		case 'h':
			unitDur = time.Hour
		case 'm':
			unitDur = time.Minute
		default:
			unitDur = 0
		}
		if unitDur > 0 && strings.Trim(numPart, "0123456789") == "" {
			var n int
			if _, err := fmt.Sscanf(numPart, "%d", &n); err != nil {
				return 0, fmt.Errorf("bad relative time %q", raw)
			}
			return now.Add(-time.Duration(n) * unitDur).UnixMilli(), nil
		}
	}

	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if ts, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return ts.UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("bad time value %q (want 2026-08-01[THH:MM[:SS]] or 7d/12h/30m)", raw)
}

// prependArgs returns first followed by rest, so callers can keep the
// leading MATCH ? parameter in position while appending window/limit args.
func prependArgs(first string, rest ...any) []any {
	out := make([]any, 0, len(rest)+1)
	out = append(out, first)
	out = append(out, rest...)
	return out
}

// prependAny is prependArgs for a non-string leading arg (the serialized
// query vector blob).
func prependAny(first any, rest ...any) []any {
	out := make([]any, 0, len(rest)+1)
	out = append(out, first)
	out = append(out, rest...)
	return out
}
