package update

import (
	"os"
	"path"
	"strings"
)

// DefaultFeeds, in try order after any user mirrors:
// R2 first (reachable from mainland China), GitHub second.
var DefaultFeeds = []string{
	"https://releases.re-mem-ber.me/latest.json",
	"https://github.com/colinleefish/rmb-desktop/releases/latest/download/manifest.json",
}

// Feeds resolves the feed list: RMB_UPDATE_FEED (explicit override, testing /
// power users) → user mirrors from config.yaml → defaults.
func Feeds(mirrors []string) []string {
	if f := strings.TrimSpace(os.Getenv("RMB_UPDATE_FEED")); f != "" {
		return []string{f}
	}
	out := make([]string, 0, len(mirrors)+len(DefaultFeeds))
	out = append(out, mirrors...)
	return append(out, DefaultFeeds...)
}

// BundleURL derives the absolute download URL for a manifest artifact: the
// feed's directory plus the artifact's file name. Bundles are stored FLAT on
// every host (the file name itself carries the version), so a single signed
// manifest resolves identically on R2 (https://host/), GitHub
// (.../releases/latest/download/), and user mirrors.
func BundleURL(feedURL, file string) string {
	dir := feedURL
	for _, name := range []string{"latest.json", "manifest.json"} {
		if strings.HasSuffix(dir, "/"+name) {
			dir = strings.TrimSuffix(dir, name)
			break
		}
	}
	if dir == feedURL { // unusual feed name: fall back to its directory
		dir = path.Dir(feedURL)
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
	}
	return dir + file
}
