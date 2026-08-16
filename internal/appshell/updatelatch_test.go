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
