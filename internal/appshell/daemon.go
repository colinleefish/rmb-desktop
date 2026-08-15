package appshell

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/platform"
	"github.com/colinleefish/rmb-desktop/internal/version"
)

// launchdLabel is the legacy launch agent label rmbd may be running under;
// the shell detaches it so only its own managed child serves.
const launchdLabel = "me.remember.rmbd"

// DaemonManager supervises the rmbd sidecar process. Port of daemon.rs.
type DaemonManager struct {
	mu      sync.Mutex
	child   *exec.Cmd
	childCh chan struct{} // closed when the child has been reaped

	rmbdPath string

	shuttingDown atomic.Bool

	expectedVersion string
	expectedCommit  string
}

// NewDaemonManager resolves the rmbd binary and stamps the expected
// version/commit from the linked-in build info (set via -ldflags by make).
func NewDaemonManager() *DaemonManager {
	return &DaemonManager{
		rmbdPath:        findRmbdBinary(),
		expectedVersion: version.Version,
		expectedCommit:  version.Commit,
	}
}

// RmbdPath is the resolved daemon binary (dev/diagnostics).
func (d *DaemonManager) RmbdPath() string { return d.rmbdPath }

// ManagedRunning reports whether our own child is alive. Port of
// managed_running (which reaps a finished child via try_wait; here the wait
// goroutine reaps and we consume its completion).
func (d *DaemonManager) ManagedRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.child == nil {
		return false
	}
	select {
	case <-d.childCh:
		d.child = nil
		d.childCh = nil
		return false
	default:
		return true
	}
}

// Start spawns the managed rmbd. Port of start().
func (d *DaemonManager) Start() error {
	if d.shuttingDown.Load() {
		return errors.New("shutting down")
	}
	if d.ManagedRunning() {
		return nil
	}

	d.mu.Lock()
	if d.child != nil { // someone raced us to it
		d.mu.Unlock()
		return nil
	}
	cmd := exec.Command(d.rmbdPath, d.serveArgs()...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("failed to start rmbd at %s: %w", d.rmbdPath, err)
	}
	ch := make(chan struct{})
	d.child = cmd
	d.childCh = ch
	d.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		close(ch)
	}()
	return nil
}

// StopManaged kills the managed child, if any. Port of stop_managed.
func (d *DaemonManager) StopManaged() {
	d.mu.Lock()
	cmd := d.child
	ch := d.childCh
	d.child = nil
	d.childCh = nil
	d.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	if ch != nil {
		<-ch
	}
}

// EnsureRunning starts the managed daemon, replacing any stale/external
// listener. Port of ensure_running.
func (d *DaemonManager) EnsureRunning() error {
	if d.shuttingDown.Load() {
		return nil
	}
	if d.ManagedRunning() {
		if d.runningMatchesExpected() {
			return nil
		}
		// Managed child is an old build — recycle it.
		d.StopManaged()
	} else if HealthOK(BaseURL()) && d.runningMatchesExpected() {
		// External daemon already serves this build.
		return nil
	}

	detachExternalDaemon()
	waitForHealth(false, 3*time.Second)
	if d.shuttingDown.Load() {
		return nil
	}

	if err := d.Start(); err != nil {
		return err
	}
	waitForHealth(true, 15*time.Second)
	if d.shuttingDown.Load() {
		return nil
	}
	if !HealthOK(BaseURL()) {
		return fmt.Errorf("rmbd did not become healthy at %s", BaseURL())
	}
	return nil
}

// RestartAfterUpdate always restarts so a newly installed binary is what
// serves. Port of restart_after_update.
func (d *DaemonManager) RestartAfterUpdate() error {
	if d.shuttingDown.Load() {
		return nil
	}
	d.StopManaged()
	detachExternalDaemon()
	waitForHealth(false, 3*time.Second)
	if d.shuttingDown.Load() {
		return nil
	}
	if err := d.Start(); err != nil {
		return err
	}
	waitForHealth(true, 15*time.Second)
	if !HealthOK(BaseURL()) {
		return fmt.Errorf("rmbd did not become healthy at %s after update restart", BaseURL())
	}
	return nil
}

// Shutdown stops the managed daemon, unloads launchd, and kills listeners on
// the configured port. Port of shutdown.
func (d *DaemonManager) Shutdown() {
	d.shuttingDown.Store(true)
	d.StopManaged()
	detachExternalDaemon()
	waitForHealth(false, 5*time.Second)
	// Last resort: anything still on the port.
	killListenersOnPort(DaemonPort())
	waitForHealth(false, 2*time.Second)
}

func (d *DaemonManager) runningMatchesExpected() bool {
	remote, ok := fetchVersion(BaseURL())
	if !ok {
		return false
	}
	return versionMatches(d.expectedVersion, d.expectedCommit, remote)
}

func fetchVersion(base string) (versionPayload, bool) {
	url := strings.TrimRight(base, "/") + "/api/v1/version"
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return versionPayload{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return versionPayload{}, false
	}
	var p versionPayload
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return versionPayload{}, false
	}
	return p, true
}

func waitForHealth(wantHealthy bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if HealthOK(BaseURL()) == wantHealthy {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// serveArgs mirrors config_serve_args: pass -config when config.yaml exists.
func (d *DaemonManager) serveArgs() []string {
	args := []string{"serve"}
	if path, err := platform.ConfigPath(); err == nil && isFile(path) {
		args = append(args, "-config", path)
	}
	return args
}

// findRmbdBinary ports find_rmbd_binary: RMBD_PATH env → installed daemon →
// candidates next to the exe (incl. repo bin/ dev fallbacks) → PATH → "rmbd".
func findRmbdBinary() string {
	if p := strings.TrimSpace(os.Getenv("RMBD_PATH")); p != "" {
		if isFile(p) {
			return p
		}
	}
	if p := InstalledDaemonPath(); isFile(p) {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{
			"rmbd", "rmbd.exe", "rmbd-desktop", "rmbd-desktop.exe",
			"../rmbd", "../../../bin/rmbd", "../../../../bin/rmbd",
		} {
			p := filepath.Join(dir, cand)
			if isFile(p) {
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}
		}
	}
	if p, ok := whichRmbd(); ok {
		return p
	}
	return "rmbd"
}

func whichRmbd() (string, bool) {
	lookup := "which"
	if runtime.GOOS == "windows" {
		lookup = "where"
	}
	out, err := exec.Command(lookup, "rmbd").Output()
	if err != nil {
		return "", false
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", false
	}
	return p, true
}
