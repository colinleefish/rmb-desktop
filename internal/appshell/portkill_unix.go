//go:build unix

package appshell

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// killListenersOnPort ports the unix (lsof) branch. TERM, grace, KILL.
func killListenersOnPort(port uint16) {
	out, err := exec.Command("lsof", "-ti", ":"+strconv.Itoa(int(port))).Output()
	if err != nil {
		return
	}
	pids := strings.Fields(string(out))
	for _, pid := range pids {
		_ = exec.Command("kill", "-TERM", pid).Run()
	}
	time.Sleep(400 * time.Millisecond)
	for _, pid := range pids {
		_ = exec.Command("kill", "-KILL", pid).Run()
	}
	time.Sleep(200 * time.Millisecond)
}

func stringTrimSpace(s string) string { return strings.TrimSpace(s) }
