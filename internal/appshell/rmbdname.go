package appshell

import "strings"

// looksLikeRmbd reports whether a process image/command name is part of the
// rmbd family (rmbd, rmbd-desktop, rmbd.exe, rmbd-desktop.exe, and the .bak/
// .old suffixes a swap-in-progress may briefly leave behind). Used to guard
// killListenersOnPort so a last-resort cleanup can never take out an
// unrelated process that happens to be squatting on the daemon's port.
func looksLikeRmbd(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "rmbd", "rmbd-desktop":
		return true
	default:
		return false
	}
}
