//go:build !darwin

package appshell

// RunLoginItemCoordinator is a no-op off macOS: login items only exist
// there, and the SMAppService bridge is darwin-only.
func RunLoginItemCoordinator() {}
