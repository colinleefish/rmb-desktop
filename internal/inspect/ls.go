package inspect

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Default page size for ls listings.
const defaultLsLimit = 200

// maxLsLimit caps a single page to keep the daemon responsive.
const maxLsLimit = 10000

// LsOptions controls paging and filtering for ls listings.
// Since/Until are inclusive bounds in unix milliseconds applied to the
// container's time column (updated_at; session_turns uses created_at).
type LsOptions struct {
	Limit  int
	Offset int
	Since  int64
	Until  int64
	Count  bool
}

// DefaultLsOptions returns the historical default behavior: 200 rows,
// no offset, no time filter.
func DefaultLsOptions() LsOptions {
	return LsOptions{Limit: defaultLsLimit}
}

func (o LsOptions) normalized() (limit, offset int) {
	limit = o.Limit
	if limit <= 0 {
		limit = defaultLsLimit
	}
	if limit > maxLsLimit {
		limit = maxLsLimit
	}
	offset = o.Offset
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

var relativeTimeRE = regexp.MustCompile(`^(\d+)([smhdw])$`)

// ParseTimeFilter accepts an absolute date ("2026-08-01"), an RFC3339
// timestamp, or a relative duration like "7d", "30d", "12h", "2w" —
// relative values are resolved against now. Returns unix milliseconds.
func ParseTimeFilter(s string, now time.Time) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty time filter")
	}
	if m := relativeTimeRE.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("bad time filter %q", s)
		}
		var unit time.Duration
		switch m[2] {
		case "s":
			unit = time.Second
		case "m":
			unit = time.Minute
		case "h":
			unit = time.Hour
		case "d":
			unit = 24 * time.Hour
		case "w":
			unit = 7 * 24 * time.Hour
		}
		return now.Add(-time.Duration(n) * unit).UnixMilli(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t.UnixMilli(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixMilli(), nil
	}
	return 0, fmt.Errorf("bad time filter %q (want 2026-08-01, RFC3339, or relative like 7d)", s)
}

// likePrefix escapes LIKE wildcards in a prefix term so segment characters
// such as '_' or '%' match literally. The caller must pair it with
// ESCAPE '\' in SQL.
func likePrefix(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s) + "%"
}

// queryList runs a parameterized listing query:
//
//	SELECT <selCol> FROM <table> WHERE <conds> ORDER BY <orderCol> LIMIT ? OFFSET ?
//
// Filters are always composed into the WHERE clause — never appended after
// LIMIT (the bug fixed by this helper's introduction). Returns the page of
// values and, when opts.Count is set, the total matching rows (else -1).
func (s *Service) queryList(ctx context.Context, table, selCol string, conds []string, args []any, orderCol, timeCol string, opts LsOptions) ([]string, int, error) {
	conds = append([]string(nil), conds...)
	args = append([]any(nil), args...)
	if len(conds) == 0 {
		conds = append(conds, "1=1")
	}
	if opts.Since > 0 {
		conds = append(conds, timeCol+" >= ?")
		args = append(args, opts.Since)
	}
	if opts.Until > 0 {
		conds = append(conds, timeCol+" <= ?")
		args = append(args, opts.Until)
	}
	where := strings.Join(conds, " AND ")

	total := -1
	if opts.Count {
		if err := s.db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where), args...,
		).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	limit, offset := opts.normalized()
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT ? OFFSET ?",
			selCol, table, where, orderCol),
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// writeLsResult prints the optional "showing X-Y of Z" header (when Count is
// set) followed by one formatted line per item, preserving the historical
// one-uri-per-line output when no new flags are used.
func writeLsResult(w io.Writer, items []string, total int, opts LsOptions, line func(string) string) error {
	if opts.Count {
		if len(items) > 0 {
			if _, err := fmt.Fprintf(w, "showing %d-%d of %d\n", opts.Offset+1, opts.Offset+len(items), total); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "showing 0 of %d\n", total); err != nil {
				return err
			}
		}
	}
	for _, item := range items {
		if _, err := fmt.Fprintln(w, line(item)); err != nil {
			return err
		}
	}
	return nil
}
