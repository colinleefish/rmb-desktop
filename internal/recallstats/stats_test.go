package recallstats

import "testing"

func TestNormalizeURI(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"rmb://profile", "rmb://profile", true},
		{"rmb://entities/jenkins", "rmb://entities/jenkins", true},
		{"rmb://scenes/11111111-1111-4111-8111-111111111111", "rmb://scenes/11111111-1111-4111-8111-111111111111", true},
		{"rmb://skills/jump-hs99-vip/scripts/run.sh", "rmb://skills/jump-hs99-vip", true},
		{"rmb://sessions/foo", "", false},
	}
	for _, tc := range tests {
		got, ok := NormalizeURI(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("NormalizeURI(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}
