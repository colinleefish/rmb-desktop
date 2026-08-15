package db

import "strings"

// NullIfEmpty maps an empty (or whitespace-only) string to SQL NULL for use
// as a scan/exec argument.
func NullIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
