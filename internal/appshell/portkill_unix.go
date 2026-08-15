//go:build unix

package appshell

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// killListenersOnPort ports the unix (lsof) branch, hardened: only PIDs whose
// process name looks like an rmbd binary are touched. This is a last-resort
// cleanup during Shutdown/EnsureRunning — it must never kill an unrelated
// process (a browser, another app) that happens to hold the configured port.
func killListenersOnPort(port uint16) {
	out, err := exec.Command("lsof", "-ti", ":"+strconv.Itoa(int(port))).Output()
	if err != nil {
		return
	}
	pids := filterRmbdPIDs(strings.Fields(string(out)))
	if len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		_ = exec.Command("kill", "-TERM", pid).Run()
	}
	time.Sleep(400 * time.Millisecond)
	for _, pid := range pids {
		_ = exec.Command("kill", "-KILL", pid).Run()
	}
	time.Sleep(200 * time.Millisecond)
}

// filterRmbdPIDs keeps only PIDs whose command name matches the rmbd family
// (rmbd, rmbd-desktop, ...). Anything else is logged and left alone.
func filterRmbdPIDs(pids []string) []string {
	var keep []string
	for _, pid := range pids {
		name, err := processName(pid)
		if err != nil {
			stderrPrintf("port-kill: skip pid %s (could not inspect: %v)\n", pid, err)
			continue
		}
		if !looksLikeRmbd(name) {
			stderrPrintf("port-kill: skip pid %s (%q is not rmbd)\n", pid, name)
			continue
		}
		keep = append(keep, pid)
	}
	return keep
}

func processName(pid string) (string, error) {
	out, err := exec.Command("ps", "-p", pid, "-o", "comm=").Output()
	if err != nil {
		return "", err
	}
	// macOS ps prints the full executable path in comm=; Linux prints a
	// short name. Normalize to a basename so looksLikeRmbd's contract stays
	// a bare process/image name on every platform.
	return filepath.Base(stringTrimSpace(string(out))), nil
}

func stringTrimSpace(s string) string { return strings.TrimSpace(s) }
