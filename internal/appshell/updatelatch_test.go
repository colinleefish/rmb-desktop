package appshell

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

// TestUpdateStopThenRestartRespawnsDaemon is the regression test for the
// 2026-08-16 v0.2.4 incident: installUpdate stopped the daemon via
// Shutdown(), whose shuttingDown latch is never cleared — so the
// RestartAfterUpdate that followed the sidecar swap was a silent no-op.
// Symptom: sidecars updated to 0.2.4 but rmbd never came back until the app
// itself was relaunched. The fixed flow stops via StopForUpdate (a separate
// updating latch that RestartAfterUpdate clears) and must respawn.
func TestUpdateStopThenRestartRespawnsDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake daemon is a unix shell script")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Fake health endpoint: healthy ⇔ the atomic flag, letting the test skip
	// the unhealthy-wait budgets inside StopForUpdate/RestartAfterUpdate.
	var healthy atomic.Bool
	healthy.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" && healthy.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("RMB_ADDR", srv.URL)

	script := filepath.Join(home, "fake-rmbd")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatalf("write fake daemon: %v", err)
	}
	t.Setenv("RMBD_PATH", script)

	d := NewDaemonManager()
	if err := d.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !d.ManagedRunning() {
		t.Fatal("fake daemon should be running after Start")
	}

	// The update flow: stop for swap, with the daemon "going down".
	healthy.Store(false)
	d.StopForUpdate()
	if d.ManagedRunning() {
		t.Fatal("daemon must be stopped for the sidecar swap")
	}
	// The tray health poller must not respawn the old binary mid-swap.
	if err := d.Start(); err == nil || !strings.Contains(err.Error(), "update in progress") {
		t.Fatalf("Start during update must fail with update in progress, got %v", err)
	}
	if err := d.EnsureRunning(); err == nil || !strings.Contains(err.Error(), "update in progress") {
		t.Fatalf("EnsureRunning during update must fail with update in progress, got %v", err)
	}

	// Swap done, new binary "up": the restart must actually spawn.
	healthy.Store(true)
	if err := d.RestartAfterUpdate(); err != nil {
		t.Fatalf("restart after update: %v", err)
	}
	if !d.ManagedRunning() {
		t.Fatal("REGRESSION: daemon must respawn after RestartAfterUpdate")
	}
	// And the poller may assist again afterwards.
	if err := d.EnsureRunning(); err != nil {
		t.Fatalf("EnsureRunning after update: %v", err)
	}
	d.StopManaged()
}

// TestSpawnedDaemonStdioIsLogFdNotPipe is the regression guard for the
// 2026-08-16 v0.2.5 headless-install incident: the daemon's stdout/stderr
// were pipes owned by the spawning shell. The headless installer exits
// right after the swap — the next daemon log write then hit EPIPE and the
// Go runtime SIGPIPE-killed the freshly updated daemon. The child must
// hold the log file fd directly so it outlives its parent.
func TestSpawnedDaemonStdioIsLogFdNotPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake daemon is a unix shell script")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	script := filepath.Join(home, "fake-rmbd")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho daemon-mark\nsleep 60\n"), 0o755); err != nil {
		t.Fatalf("write fake daemon: %v", err)
	}
	t.Setenv("RMBD_PATH", script)

	d := NewDaemonManager()
	if err := d.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer d.StopManaged()

	d.mu.Lock()
	stdout, stderr := d.child.Stdout, d.child.Stderr
	d.mu.Unlock()
	for _, io := range []struct {
		name string
		v    any
	}{{"stdout", stdout}, {"stderr", stderr}} {
		if _, ok := io.v.(*os.File); !ok {
			t.Errorf("daemon %s must be the log *os.File (direct fd), got %T", io.name, io.v)
		}
	}

	// The child's own write must land in the log file with no parent pipe.
	path, err := platform.DaemonLogPath()
	if err != nil {
		t.Fatalf("log path: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "daemon-mark") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("child write via direct fd never reached the daemon log")
}

// TestShutdownStillBlocksRestart pins the app-quit semantics: after
// Shutdown, RestartAfterUpdate stays a no-op (returning nil) so a Quit
// racing an update cannot resurrect the daemon.
func TestShutdownStillBlocksRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake daemon is a unix shell script")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	var healthy atomic.Bool
	healthy.Store(false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("RMB_ADDR", srv.URL)

	script := filepath.Join(home, "fake-rmbd")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatalf("write fake daemon: %v", err)
	}
	t.Setenv("RMBD_PATH", script)

	d := NewDaemonManager()
	d.Shutdown()
	if err := d.RestartAfterUpdate(); err != nil {
		t.Fatalf("quit-path restart must be a no-op, got %v", err)
	}
	if d.ManagedRunning() {
		t.Fatal("daemon must not respawn after Shutdown")
	}
}
