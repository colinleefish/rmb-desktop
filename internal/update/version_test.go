package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		remote, current string
		want            bool
	}{
		{"0.2.0", "0.1.22", true},
		{"v0.2.0", "0.1.22", true},
		{"0.2.1", "0.2.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"0.1.22", "0.1.22", false},
		{"0.1.21", "0.1.22", false},
		{"0.1.9", "0.1.10", false},
		{"0.2.0-beta", "0.1.22", false}, // prerelease never auto-installs
		{"0.2.0", "0.2.0-beta", true},   // release beats its prerelease
		{"0.2", "0.1.22", true},
		{"0.2.0.1", "0.2.0", false}, // 4 components are invalid, not newer
	}
	for _, tc := range cases {
		got, err := IsNewer(tc.remote, tc.current)
		if tc.remote == "0.2.0.1" {
			if err == nil {
				t.Errorf("IsNewer(%q,%q): want error", tc.remote, tc.current)
			}
			continue
		}
		if err != nil {
			t.Errorf("IsNewer(%q,%q): %v", tc.remote, tc.current, err)
			continue
		}
		if got != tc.want {
			t.Errorf("IsNewer(%q,%q) = %v, want %v", tc.remote, tc.current, got, tc.want)
		}
	}
}

func TestBundleURL(t *testing.T) {
	cases := []struct{ feed, version, file, want string }{
		{
			"https://releases.re-mem-ber.me/latest.json",
			"0.2.0", "b.tar.gz",
			"https://releases.re-mem-ber.me/0.2.0/b.tar.gz",
		},
		{
			"https://github.com/c/rd/releases/latest/download/manifest.json",
			"0.2.0", "b.zip",
			"https://github.com/c/rd/releases/latest/download/0.2.0/b.zip",
		},
		{
			"https://mirror.cn/rmb/manifest.json",
			"0.2.0", "b.tar.gz",
			"https://mirror.cn/rmb/0.2.0/b.tar.gz",
		},
	}
	for _, tc := range cases {
		if got := BundleURL(tc.feed, tc.version, tc.file); got != tc.want {
			t.Errorf("BundleURL(%q) = %q, want %q", tc.feed, got, tc.want)
		}
	}
}

func TestFeeds(t *testing.T) {
	t.Setenv("RMB_UPDATE_FEED", "https://test/feed.json")
	got := Feeds([]string{"https://m1/x"})
	if len(got) != 1 || got[0] != "https://test/feed.json" {
		t.Errorf("env override: %v", got)
	}

	t.Setenv("RMB_UPDATE_FEED", "")
	got = Feeds([]string{"https://m1/x", "https://m2/y"})
	if len(got) != 4 || got[0] != "https://m1/x" || got[1] != "https://m2/y" || got[2] != DefaultFeeds[0] {
		t.Errorf("mirrors: %v", got)
	}
}
