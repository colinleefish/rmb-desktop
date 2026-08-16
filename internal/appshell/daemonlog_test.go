package appshell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

func TestOpenDaemonLogFileRotatesOversized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "rmbd.log")

	w := openDaemonLogFile(path, 64)
	if w == nil {
		t.Fatal("expected log writer, got nil")
	}
	if _, err := w.Write([]byte(strings.Repeat("x", 100))); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()

	w = openDaemonLogFile(path, 64)
	if w == nil {
		t.Fatal("expected reopened log writer, got nil")
	}
	_ = w.Close()

	if _, err := os.Stat(path + ".old"); err != nil {
		t.Errorf("oversized log must rotate to .old: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("rotated-in log missing: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("rotated-in log must be empty, got size=%d", info.Size())
	}
}

func TestOpenDaemonLogPurgesAged(t *testing.T) {
	old := daemonLogRetention
	daemonLogRetention = 7 * 24 * time.Hour
	defer func() { daemonLogRetention = old }()

	path := filepath.Join(t.TempDir(), "logs", "rmbd.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-8 * 24 * time.Hour)
	for _, p := range []string{path, path + ".old"} {
		if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stale, stale); err != nil {
			t.Fatal(err)
		}
	}

	w := openDaemonLogFile(path, 1<<20)
	if w == nil {
		t.Fatal("expected log writer, got nil")
	}
	_, _ = w.Write([]byte("fresh"))
	_ = w.Close()

	// The stale .old is purged and not recreated: with a small fresh log
	// there is nothing to rotate.
	if _, err := os.Stat(path + ".old"); !os.IsNotExist(err) {
		t.Errorf("stale .old must be purged, stat err=%v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "fresh" {
		t.Errorf("log must start anew, got %q err=%v", data, err)
	}
}

func TestDaemonLogWriterRotatesMidStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "rmbd.log")
	w := openDaemonLogFile(path, 32)
	if w == nil {
		t.Fatal("expected log writer, got nil")
	}

	// Write well past the cap in one session — rotation must happen live,
	// without a reopen, so a long-lived daemon is bounded too.
	for i := 0; i < 10; i++ {
		if _, err := fmt.Fprintf(w, "line-%02d\n", i); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	_ = w.Close()

	old, err := os.ReadFile(path + ".old")
	if err != nil {
		t.Fatalf("mid-stream rotation must produce .old: %v", err)
	}
	// Only the last ~2×cap bytes are kept: .old holds the previous
	// generation, never the full history.
	if strings.Contains(string(old), "line-00") || strings.Contains(string(old), "line-09") {
		t.Errorf(".old must hold a middle generation, got %q", old)
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(cur), "line-09\n") {
		t.Errorf("current log must hold the latest lines, got %q", cur)
	}
	if len(cur) > 64 {
		t.Errorf("current log must stay near the cap, got %d bytes", len(cur))
	}
}

func TestDaemonLogWriterConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "rmbd.log")
	w := openDaemonLogFile(path, 256)
	if w == nil {
		t.Fatal("expected log writer, got nil")
	}
	defer w.Close()

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_, _ = fmt.Fprintf(w, "goroutine line %d\n", i)
			}
		}()
	}
	wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		t.Fatal("writer must survive concurrent rotation")
	}
	for _, p := range []string{path, path + ".old"} {
		if info, err := os.Stat(p); err == nil && info.Size() > 512 {
			t.Errorf("%s exceeds the ~2x cap: %d bytes", p, info.Size())
		}
	}
}

// TestStartRecordsExitAndLogsOutput reproduces the diagnostics gap from the
// 2026-08-16 incident: a daemon that dies instantly must (a) have its
// stderr/stdout captured in the daemon log and (b) surface the failure via
// LastError for the tray, instead of vanishing silently into io.Discard.
func TestStartRecordsExitAndLogsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no /bin/sh on windows")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RMBD_PATH", "/bin/sh") // sh "serve" exits 127 immediately

	d := NewDaemonManager()
	if err := d.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for d.ManagedRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	for d.LastError() == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if msg := d.LastError(); msg == "" {
		t.Error("instant daemon exit must be recorded in LastError")
	} else if !strings.Contains(msg, "rmbd exited after") {
		t.Errorf("unexpected LastError: %q", msg)
	}

	path, err := platform.DaemonLogPath()
	if err != nil {
		t.Fatalf("log path: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("daemon log missing: %v", err)
	}
	log := string(data)
	for _, want := range []string{
		"spawn /bin/sh serve", // spawn header
		"serve: No such file", // daemon stderr captured
		"exit after ",         // exit footer with duration
	} {
		if !strings.Contains(log, want) {
			t.Errorf("daemon log missing %q; log:\n%s", want, log)
		}
	}
}
