//go:build darwin

package launchatlogin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withNeutralizedInstall points the /Applications candidate at a nonexistent
// path under the test HOME so appBinaryPath resolution is independent of
// whether RMB Desktop is actually installed on the host. Restored on cleanup.
func withNeutralizedInstall(t *testing.T, home string) {
	t.Helper()
	prev := installedAppPath
	installedAppPath = filepath.Join(home, "nonexistent-app")
	t.Cleanup(func() { installedAppPath = prev })
}

func TestWritePlist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withNeutralizedInstall(t, home)

	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rmb-app"), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rmbd-desktop"), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	if err := writePlist(plistPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, label) {
		t.Fatalf("missing label: %s", text)
	}
	if !strings.Contains(text, filepath.Join(binDir, "rmb-app")) {
		t.Fatalf("missing app binary: %s", text)
	}
}

// TestAppBinaryPath locks in the resolution priority host-independently:
//  1. /Applications install present → preferred
//  2. otherwise ~/.rmb/bin/rmb-app present → used
//  3. otherwise → ~/.rmb/bin/rmb fallback (need not exist)
func TestAppBinaryPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prev := installedAppPath
	t.Cleanup(func() { installedAppPath = prev })

	// 1. Installed app present → preferred over ~/.rmb/bin/rmb-app.
	installedAppPath = filepath.Join(home, "fake-app")
	if err := os.WriteFile(installedAppPath, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	rmbApp := filepath.Join(binDir, "rmb-app")
	if err := os.WriteFile(rmbApp, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := appBinaryPath(home); got != installedAppPath {
		t.Fatalf("installed app preferred: got %s, want %s", got, installedAppPath)
	}

	// 2. Installed app absent → fall back to ~/.rmb/bin/rmb-app.
	if err := os.Remove(installedAppPath); err != nil {
		t.Fatal(err)
	}
	installedAppPath = filepath.Join(home, "nonexistent-app")
	if got := appBinaryPath(home); got != rmbApp {
		t.Fatalf("rmb-app fallback: got %s, want %s", got, rmbApp)
	}

	// 3. Neither present → rmb fallback path (need not exist on disk).
	if err := os.Remove(rmbApp); err != nil {
		t.Fatal(err)
	}
	rmb := filepath.Join(binDir, "rmb")
	if got := appBinaryPath(home); got != rmb {
		t.Fatalf("rmb fallback: got %s, want %s", got, rmb)
	}
}
