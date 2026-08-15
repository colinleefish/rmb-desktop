//go:build unix

package appshell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// AcquireInstanceLock ports instance.rs: an flock on ~/.rmb/rmb-app.lock.
// The returned release func must be called before exit; a lock held by
// another process yields ErrAlreadyRunning.
var ErrAlreadyRunning = errors.New("RMB is already running")

func AcquireInstanceLock() (func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory not found: %w", err)
	}
	path := filepath.Join(home, ".rmb", "rmb-app.lock")
	return acquireLockAt(path)
}

func acquireLockAt(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, ErrAlreadyRunning
	}
	if err := f.Truncate(0); err == nil {
		_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	}

	release := func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}
