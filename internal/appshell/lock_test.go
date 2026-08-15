//go:build unix

package appshell

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAcquireInstanceLockCrossProcess holds a lock at a temp path, then
// re-execs the test binary as a child that must fail to acquire the same
// path. (The real ~/.rmb path may be held by the user's running shell.)
func TestAcquireInstanceLockCrossProcess(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "rmb-app.lock")

	if path := os.Getenv("RMB_TEST_LOCK_PATH"); path != "" {
		if _, err := acquireLockAt(path); err == nil {
			os.Exit(1) // wrongly acquired
		} else if err != ErrAlreadyRunning {
			os.Exit(2) // unexpected error
		}
		os.Exit(0)
	}

	release, err := acquireLockAt(lockPath)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()

	cmd := exec.Command(os.Args[0], "-test.run=TestAcquireInstanceLockCrossProcess")
	cmd.Env = append(os.Environ(), "RMB_TEST_LOCK_PATH="+lockPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child should be refused the lock; exit=%v output=%s", err, out)
	}
}
