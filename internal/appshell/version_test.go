package appshell

import "testing"

// versionMatches is the recycle predicate ported from daemon.rs
// running_matches_expected.
func TestVersionMatches(t *testing.T) {
	cases := []struct {
		name       string
		expV, expC string
		remV, remC string
		want       bool
	}{
		{"exact match", "0.1.21", "abc1234", "0.1.21", "abc1234", true},
		{"version differs", "0.1.21", "abc1234", "0.1.20", "abc1234", false},
		{"expected commit unknown skips check", "0.1.21", "unknown", "0.1.21", "abc1234", true},
		{"expected commit dev skips check (go default)", "0.1.21", "dev", "0.1.21", "abc1234", true},
		{"remote commit unknown skips check", "0.1.21", "abc1234", "0.1.21", "", true},
		{"remote commit dev skips check", "0.1.21", "abc1234", "0.1.21", "dev", true},
		{"commit differs", "0.1.21", "abc1234", "0.1.21", "def5678", false},
		{"dirty rebuild same version recycles", "0.1.21", "abc1234", "0.1.21", "abc1234-dirty", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := versionPayload{Version: tc.remV, Commit: tc.remC}
			if got := versionMatches(tc.expV, tc.expC, remote); got != tc.want {
				t.Errorf("versionMatches(%q,%q,%+v) = %v, want %v", tc.expV, tc.expC, remote, got, tc.want)
			}
		})
	}
}
