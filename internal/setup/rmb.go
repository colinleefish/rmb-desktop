package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RMBPath resolves the rmb binary used in hook commands.
func RMBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		for _, candidate := range []string{
			filepath.Join(home, ".rmb", "bin", "rmb"),
			filepath.Join(home, ".rmb", "bin", "rmb-desktop"),
		} {
			if st, statErr := os.Stat(candidate); statErr == nil && !st.IsDir() {
				return candidate, nil
			}
		}
	}
	if p, err := exec.LookPath("rmb"); err == nil {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		if base := strings.TrimSpace(filepath.Base(exe)); base == "rmb" || base == "rmb-desktop" {
			return exe, nil
		}
	}
	return "", os.ErrNotExist
}

func hookCommand(source string) (string, error) {
	bin, err := RMBPath()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(bin) + " hook-submit --source=" + source, nil
}

func isRMBHookCommand(cmd string) bool {
	c := strings.ToLower(strings.TrimSpace(cmd))
	return strings.Contains(c, "rmb hook-submit")
}
