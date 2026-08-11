//go:build !darwin

package launchatlogin

import "fmt"

func set(enabled bool) error {
	if enabled {
		return fmt.Errorf("launch at login is only supported on macOS")
	}
	return nil
}
