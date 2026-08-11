//go:build darwin

package launchatlogin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePlist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
