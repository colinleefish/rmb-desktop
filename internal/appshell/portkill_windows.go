//go:build windows

package appshell

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// killListenersOnPort ports the Windows (netstat) branch: find LISTENING
// PIDs on the port and taskkill them.
func killListenersOnPort(port uint16) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return
	}
	needle := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, needle) || !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pid := fields[len(fields)-1]
		_ = exec.Command("taskkill", "/PID", pid, "/F").Run()
	}
	time.Sleep(300 * time.Millisecond)
}
