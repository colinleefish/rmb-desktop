package update

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// IsNewer reports whether remote > current under a pragmatic semver
// comparison: optional "v" prefix, numeric major.minor.patch, and an optional
// "-" suffix (prerelease), where any prerelease sorts below its release.
func IsNewer(remote, current string) (bool, error) {
	r, err := parseVersion(remote)
	if err != nil {
		return false, fmt.Errorf("update: bad remote version %q: %w", remote, err)
	}
	c, err := parseVersion(current)
	if err != nil {
		return false, fmt.Errorf("update: bad current version %q: %w", current, err)
	}
	// Policy: prereleases are never auto-installed, regardless of numbers.
	if r.pre != "" {
		return false, nil
	}
	if r.nums != c.nums {
		return r.nums.compare(c.nums) > 0, nil
	}
	// Same numbers: a final release beats the current prerelease.
	return c.pre != "", nil
}

func parseVersion(v string) (parsed, error) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	if v == "" {
		return parsed{}, fmt.Errorf("empty")
	}
	var pre string
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return parsed{}, fmt.Errorf("expected major[.minor[.patch]]")
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return parsed{}, fmt.Errorf("non-numeric component %q", p)
		}
		nums[i] = n
	}
	return parsed{nums: nums, pre: pre}, nil
}

type parsed struct {
	nums versionTriple
	pre  string
}

type versionTriple [3]int

func (v versionTriple) compare(o versionTriple) int {
	for i := range v {
		if v[i] != o[i] {
			if v[i] > o[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// platformKey maps GOOS to the manifest platform key.
func platformKey() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
}

// archKey maps GOARCH to the manifest arch key.
func archKey() string {
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return runtime.GOARCH
}

// BundleFor picks this runtime's platform artifact from the manifest.
func BundleFor(m *Manifest) (PlatformArt, error) {
	platforms, ok := m.Platforms[platformKey()]
	if !ok {
		return PlatformArt{}, fmt.Errorf("update: no %s build in v%s", platformKey(), m.Version)
	}
	art, ok := platforms[archKey()]
	if !ok {
		return PlatformArt{}, fmt.Errorf("update: no %s/%s build in v%s", platformKey(), archKey(), m.Version)
	}
	if art.Sidecars == "" || art.SHA256 == "" {
		return PlatformArt{}, fmt.Errorf("update: incomplete %s/%s entry in v%s", platformKey(), archKey(), m.Version)
	}
	return art, nil
}
