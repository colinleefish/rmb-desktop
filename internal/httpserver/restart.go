package httpserver

import (
	"net/http"
	"os"
	"os/exec"
	"time"
)

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	go func() {
		time.Sleep(150 * time.Millisecond)
		if err := spawnReplacement(s.configPath); err != nil {
			s.log.Error("restart spawn failed", "err", err)
			return
		}
		os.Exit(0)
	}()
}

func spawnReplacement(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"serve"}
	if configPath != "" {
		args = append(args, "-config", configPath)
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
