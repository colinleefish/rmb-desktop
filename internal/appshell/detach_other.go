//go:build !darwin

package appshell

// detachExternalDaemon ports the non-macOS branch of detach_external_daemon:
// no launchd to boot out — just kill whatever owns the port.
func detachExternalDaemon() {
	killListenersOnPort(DaemonPort())
}
