//go:build darwin

package appshell

import (
	"os"
	"os/exec"
	"path/filepath"
)

// detachExternalDaemon ports the macOS branch of detach_external_daemon:
// boot out any launchd-managed rmbd, then kill whatever still owns the port.
func detachExternalDaemon() {
	uid := guiUID()

	if uid != "" {
		domain := "gui/" + uid

		_ = launchctl("disable", domain, launchdLabel)

		if plist := launchdPlistPath(); plist != "" {
			_ = launchctl("bootout", domain, plist)
		}
		_ = launchctl("bootout", domain, launchdLabel)
	}

	killListenersOnPort(DaemonPort())
}

func launchdPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

func guiUID() string {
	out, err := exec.Command("id", "-u").Output()
	if err != nil {
		return ""
	}
	return stringTrimSpace(string(out))
}

func launchctl(args ...string) error {
	return exec.Command("launchctl", args...).Run()
}
