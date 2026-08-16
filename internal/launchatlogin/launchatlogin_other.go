//go:build !darwin

package launchatlogin

import "fmt"

func set(enabled bool) error {
	if enabled {
		return fmt.Errorf("launch at login is only supported on macOS")
	}
	return nil
}

// SetFromBundle is the macOS-only modern login-item path (SMAppService).
func SetFromBundle(enabled bool) error { return set(enabled) }

// BundleStatus is macOS-only (SMAppService status); meaningless elsewhere.
func BundleStatus() int { return 0 }

// MigrateFromLegacy is macOS-only (LaunchAgent cleanup); no-op elsewhere.
func MigrateFromLegacy() (bool, error) { return false, nil }
