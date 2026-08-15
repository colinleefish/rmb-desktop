package appshell

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsRefreshMissingDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	write(t, src, "x")

	refresh, err := needsRefresh(filepath.Join(dir, "missing-cli"), filepath.Join(dir, "missing-daemon"))
	if err != nil {
		t.Fatalf("needsRefresh: %v", err)
	}
	if !refresh {
		t.Error("missing destinations must trigger refresh")
	}
}

func TestIsNewer(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	write(t, src, "x")
	write(t, dst, "x")

	newer, err := isNewer(src, dst)
	if err != nil {
		t.Fatalf("isNewer: %v", err)
	}
	if newer {
		t.Error("same mtime must not be newer")
	}

	// Bump src mtime into the future.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	newer, err = isNewer(src, dst)
	if err != nil {
		t.Fatalf("isNewer: %v", err)
	}
	if !newer {
		t.Error("future src mtime must be newer")
	}
}

func TestFindPrefixedBinary(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "rmb-aarch64-apple-darwin"), "x")
	write(t, filepath.Join(dir, "unrelated"), "x")

	if got := findPrefixedBinary(dir, "rmb"); got == "" {
		t.Fatal("prefixed binary not found")
	} else if filepath.Base(got) != "rmb-aarch64-apple-darwin" {
		t.Errorf("found %q", got)
	}

	// Exact name wins.
	write(t, filepath.Join(dir, "rmb"), "x")
	if got := findPrefixedBinary(dir, "rmb"); filepath.Base(got) != "rmb" {
		t.Errorf("exact name should win, got %q", got)
	}

	if got := findPrefixedBinary(dir, "rmbd"); got != "" {
		t.Errorf("nothing matches rmbd, got %q", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
