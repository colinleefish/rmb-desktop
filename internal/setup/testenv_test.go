package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRMBHome points HOME at an empty temp dir containing a fake
// ~/.rmb/bin/rmb so RMBPath() resolves deterministically on any machine.
// Preview tests used to pass only on dev machines with rmb installed —
// caught by the Linux CI runner, 2026-08-29.
func fakeRMBHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rmb"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	return home
}
