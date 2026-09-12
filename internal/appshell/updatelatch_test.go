package appshell

import (
	"fmt"
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

// b04DelayedMarkScript returns the fake-daemon script for
// TestSpawnedDaemonStdioIsLogFdNotPipe. Test-only env hook
// RMB_TEST_DAEMON_WRITE_DELAY (time.ParseDuration, e.g. 8s; default unset = no
// delay) inserts a sleep before the daemon's first write, deterministically
// replaying the load-delayed child write that made B04 (issue #70) flaky: a
// child whose mark becomes visible after the test's fixed 5s poll window. // B04
func b04DelayedMarkScript(t *testing.T) string {
	t.Helper()
	script := "#!/bin/sh\necho daemon-mark\nsleep 60\n"
	if raw := os.Getenv("RMB_TEST_DAEMON_WRITE_DELAY"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < 0 {
			t.Fatalf("RMB_TEST_DAEMON_WRITE_DELAY: invalid duration %q", raw)
		}
		if d > 0 {
			script = fmt.Sprintf("#!/bin/sh\nsleep %s\necho daemon-mark\nsleep 60\n", d)
		}
	}
	return script
}

// b04MarkPollDeadline is how long TestSpawnedDaemonStdioIsLogFdNotPipe waits for the
// fake daemon's daemon-mark write. The incident (B04) used a fixed 5s window that
// false-failed under load; delay injection (RMB_TEST_DAEMON_WRITE_DELAY) needs
// base+2×delay, and the normal path adds load headroom without slowing the happy path.
func b04MarkPollDeadline(t *testing.T) time.Time {
	t.Helper()
	const base = 5 * time.Second
	var delay time.Duration
	if raw := os.Getenv("RMB_TEST_DAEMON_WRITE_DELAY"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < 0 {
			t.Fatalf("RMB_TEST_DAEMON_WRITE_DELAY: invalid duration %q", raw)
		}
		delay = d
	}
	window := base + 2*delay
	if delay == 0 {
		window += 25 * time.Second
	}
	return time.Now().Add(window)
}

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
	if err := os.WriteFile(script, []byte(b04DelayedMarkScript(t)), 0o755); err != nil {
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
	deadline := b04MarkPollDeadline(t)
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
