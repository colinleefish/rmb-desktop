//go:build windows

package appshell

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// killListenersOnPort ports the Windows (netstat) branch, hardened: only
// PIDs whose image name looks like an rmbd binary are touched. This is a
// last-resort cleanup during Shutdown/EnsureRunning — it must never kill an
// unrelated process that happens to hold the configured port.
func killListenersOnPort(port uint16) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return
	}
	needle := fmt.Sprintf(":%d", port)
	var pids []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, needle) || !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pids = append(pids, fields[len(fields)-1])
	}
	pids = filterRmbdPIDs(pids)
	for _, pid := range pids {
		_ = exec.Command("taskkill", "/PID", pid, "/F").Run()
	}
	time.Sleep(300 * time.Millisecond)
}

// filterRmbdPIDs keeps only PIDs whose image name matches the rmbd family.
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

// processName resolves the image name for pid via a CSV, no-header tasklist
// query filtered to that single PID.
func processName(pid string) (string, error) {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+pid, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	fields := strings.Split(line, ",")
	if len(fields) == 0 {
		return "", fmt.Errorf("no tasklist entry for pid %s", pid)
	}
	return strings.Trim(fields[0], `"`), nil
}
