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

// Sentinel lifecycle errors, matched by the tray poller via errors.Is.
var (
	errShuttingDown = errors.New("shutting down")
	errUpdateBusy   = errors.New("update in progress")
)

// DaemonManager supervises the rmbd sidecar process. Port of daemon.rs.
type DaemonManager struct {
	mu      sync.Mutex
	child   *exec.Cmd
	childCh chan struct{} // closed when the child has been reaped
	lastErr string        // last spawn/exit failure, surfaced via LastError()

	rmbdPath string

	shuttingDown atomic.Bool
	// updating latches while the self-updater owns daemon lifecycle (sidecar
	// swap): Start/EnsureRunning defer to it so the health poller cannot
	// respawn the old binary mid-swap. Unlike shuttingDown it is cleared by
	// RestartAfterUpdate, whose spawn must succeed.
	updating atomic.Bool

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

// LastError returns the most recent daemon spawn/exit failure for tray
// display; empty while the last spawn is presumed healthy.
func (d *DaemonManager) LastError() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastErr
}

func (d *DaemonManager) setLastErr(msg string) {
	d.mu.Lock()
	d.lastErr = msg
	d.mu.Unlock()
}

// noteDaemonError records a daemon lifecycle failure for the tray menu and
// appends it to the daemon log so it survives even without a shell console.
func (d *DaemonManager) noteDaemonError(err error) {
	d.setLastErr(err.Error())
	appendDaemonLog("=== %s error: %v ===\n", time.Now().Format(time.RFC3339), err)
}

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
		return errShuttingDown
	}
	if d.updating.Load() {
		return errUpdateBusy
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
	logFile := openDaemonLog()
	daemonOut := io.Writer(io.Discard)
	if logFile != nil {
		daemonOut = logFile
		fmt.Fprintf(logFile, "=== %s spawn %s %s ===\n",
			time.Now().Format(time.RFC3339), d.rmbdPath, strings.Join(d.serveArgs(), " "))
	}
	cmd.Stdout = daemonOut
	cmd.Stderr = daemonOut
	started := time.Now()
	if err := cmd.Start(); err != nil {
		d.mu.Unlock()
		if logFile != nil {
			fmt.Fprintf(logFile, "=== %s spawn failed: %v ===\n",
				time.Now().Format(time.RFC3339), err)
			_ = logFile.Close()
		}
		d.setLastErr(fmt.Sprintf("start rmbd at %s: %v", d.rmbdPath, err))
		return fmt.Errorf("failed to start rmbd at %s: %w", d.rmbdPath, err)
	}
	ch := make(chan struct{})
	d.child = cmd
	d.childCh = ch
	d.lastErr = ""
	d.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		// Record the exit reason before closing ch: a concurrent
		// ManagedRunning() reaps ch and clears d.child, which would otherwise
		// race away the reason for an unexpected exit.
		d.mu.Lock()
		if d.child == cmd { // unexpected exit; StopManaged clears d.child first
			reason := "clean exit"
			if waitErr != nil {
				reason = waitErr.Error()
			}
			d.lastErr = fmt.Sprintf("rmbd exited after %s: %s",
				time.Since(started).Round(time.Millisecond), reason)
		}
		d.mu.Unlock()
		close(ch)
		if logFile != nil {
			fmt.Fprintf(logFile, "=== %s exit after %s: %v ===\n",
				time.Now().Format(time.RFC3339), time.Since(started).Round(time.Millisecond), waitErr)
			_ = logFile.Close()
		}
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
	if d.updating.Load() {
		return errUpdateBusy
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
		err := fmt.Errorf("rmbd did not become healthy at %s", BaseURL())
		d.noteDaemonError(err)
		return err
	}
	return nil
}

// RestartAfterUpdate always restarts so a newly installed binary is what
// serves. Port of restart_after_update.
func (d *DaemonManager) RestartAfterUpdate() error {
	if d.shuttingDown.Load() {
		return nil
	}
	// The updater owns the daemon across a swap; clear its latch so this
	// spawn (and subsequent polls) may proceed. Regression 2026-08-16:
	// installUpdate used Shutdown(), whose shuttingDown latch made this
	// method a silent no-op — sidecars were swapped but rmbd never respawned.
	d.updating.Store(false)
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
		err := fmt.Errorf("rmbd did not become healthy at %s after update restart", BaseURL())
		d.noteDaemonError(err)
		return err
	}
	return nil
}

// Shutdown stops the managed daemon, unloads launchd, and kills listeners on
// the configured port. Port of shutdown. App-quit path only: it latches
// shuttingDown forever. The self-updater must use StopForUpdate instead.
func (d *DaemonManager) Shutdown() {
	d.shuttingDown.Store(true)
	d.StopManaged()
	detachExternalDaemon()
	waitForHealth(false, 5*time.Second)
	// Last resort: anything still on the port.
	killListenersOnPort(DaemonPort())
	waitForHealth(false, 2*time.Second)
}

// StopForUpdate stops the daemon for a sidecar swap WITHOUT latching the
// app-quit flag, so RestartAfterUpdate can bring the new binary up right
// after. While latched (updating), Start/EnsureRunning defer to the updater
// so the tray health poller cannot respawn the old binary mid-swap.
func (d *DaemonManager) StopForUpdate() {
	d.updating.Store(true)
	d.StopManaged()
	detachExternalDaemon()
	waitForHealth(false, 5*time.Second)
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

// daemonLogMaxBytes bounds the live rmbd.log; past it the log is rotated to
// rmbd.log.old (replacing any previous .old), keeping the total footprint
// near 2× the cap no matter how long the daemon lives.
var daemonLogMaxBytes int64 = 5 << 20

// daemonLogRetention is how long untouched log files are kept; any log whose
// last write predates it is removed the next time the log is opened.
var daemonLogRetention = 7 * 24 * time.Hour

// openDaemonLog opens the shared rmbd log (creating directories and the file
// as needed), purging stale files and rotating away an oversized previous
// log. It returns nil when logging is unavailable — the daemon must still
// start without a log.
func openDaemonLog() *daemonLogWriter {
	path, err := platform.DaemonLogPath()
	if err != nil {
		return nil
	}
	return openDaemonLogFile(path, daemonLogMaxBytes)
}

// openDaemonLogFile is openDaemonLog against an explicit path/size (tests).
func openDaemonLogFile(path string, max int64) *daemonLogWriter {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	purgeAgedDaemonLogs(path)
	if info, err := os.Stat(path); err == nil && info.Size() > max {
		_ = os.Remove(path + ".old")
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	var n int64
	if info, err := f.Stat(); err == nil {
		n = info.Size()
	}
	return &daemonLogWriter{path: path, max: max, f: f, n: n}
}

// purgeAgedDaemonLogs removes the log files when their last write is older
// than daemonLogRetention: a .old left behind by an old rotation, and the
// live log itself once the app has been unused for the retention window.
func purgeAgedDaemonLogs(path string) {
	if daemonLogRetention <= 0 {
		return
	}
	cutoff := time.Now().Add(-daemonLogRetention)
	for _, p := range []string{path, path + ".old"} {
		if info, err := os.Stat(p); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
	}
}

// daemonLogWriter is the daemon child's stdout/stderr sink. It appends to
// rmbd.log and rotates the file in place once it grows past max bytes, so
// the size cap holds even for a daemon that runs for weeks without a
// respawn. Safe for concurrent use: the child's stdout and stderr arrive as
// separate pipes.
type daemonLogWriter struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	n    int64 // bytes in the current file
}

func (w *daemonLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return len(p), nil // best-effort: drop rather than fail the daemon
	}
	if w.n+int64(len(p)) > w.max {
		w.rotateLocked()
		if w.f == nil {
			return len(p), nil
		}
	}
	n, err := w.f.Write(p)
	w.n += int64(n)
	return n, err
}

// rotateLocked moves the current log aside to .old and continues in a fresh
// file; on failure the writer goes dark (writes drop) rather than propagating
// errors into the daemon's output path.
func (w *daemonLogWriter) rotateLocked() {
	_ = w.f.Close()
	_ = os.Remove(w.path + ".old")
	if err := os.Rename(w.path, w.path+".old"); err != nil {
		w.f = nil
		return
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		w.f = nil
		return
	}
	w.f = f
	w.n = 0
}

func (w *daemonLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// appendDaemonLog appends one diagnostic line to the daemon log; best-effort,
// used for shell-side lifecycle errors that happen outside a live child.
func appendDaemonLog(format string, args ...any) {
	w := openDaemonLog()
	if w == nil {
		return
	}
	defer w.Close()
	fmt.Fprintf(w, format, args...)
}
