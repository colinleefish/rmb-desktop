package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/setup/integrations"
)

func TestRenderedPiExtension_usesRMBPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "rmb")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := renderedPiExtension()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, binPath) {
		t.Fatalf("expected %q in extension, got: %s", binPath, got)
	}
	if !integrations.IsRMBPiExtension(got) {
		t.Fatal("rendered extension missing rmb markers")
	}
}

func TestApplyPi_writesExtension(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "rmb")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := applyPi("extension"); err != nil {
		t.Fatal(err)
	}

	extensionPath := filepath.Join(home, ".pi", "agent", "extensions", "rmb-hook.ts")
	data, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, binPath) {
		t.Fatalf("extension missing injected binary path: %s", content)
	}
	if !integrations.IsRMBPiExtension(content) {
		t.Fatal("written file is not recognized as rmb pi extension")
	}
}

func TestPreviewPi_extensionUsesReplaceDisplay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rmb"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	def, ok := agentDefByID(AgentPi)
	if !ok {
		t.Fatal("pi agent def missing")
	}
	state, err := previewPi(def)
	if err != nil {
		t.Fatal(err)
	}
	if state.Artifacts[0].DisplayMode != DisplayReplace {
		t.Fatalf("extension display_mode = %q, want replace", state.Artifacts[0].DisplayMode)
	}
}
