package appshell

import "testing"

func TestLooksLikeRmbd(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"rmbd", true},
		{"rmbd-desktop", true},
		{"rmbd.exe", true},
		{"rmbd-desktop.exe", true},
		{"RMBD", true},
		{"  rmbd  ", true},
		{"Google Chrome Helper", false},
		{"rmb", false}, // the CLI, not the daemon — must not match
		{"rmbd-something-else", false},
		{"chrome", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeRmbd(tc.name); got != tc.want {
			t.Errorf("looksLikeRmbd(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
