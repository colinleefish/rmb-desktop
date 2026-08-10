package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/setup/integrations"
)

func TestRenderedOpenCodePlugin_usesRMBPath(t *testing.T) {
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

	got, err := renderedOpenCodePlugin()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, binPath) {
		t.Fatalf("expected %q in plugin, got: %s", binPath, got)
	}
	if !integrations.IsRMBOpenCodePlugin(got) {
		t.Fatal("rendered plugin missing rmb markers")
	}
}

func TestApplyOpenCode_writesPlugin(t *testing.T) {
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

	if err := applyOpenCode("plugin"); err != nil {
		t.Fatal(err)
	}

	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "rmb-hook.ts")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, binPath) {
		t.Fatalf("plugin missing injected binary path: %s", content)
	}
	if !integrations.IsRMBOpenCodePlugin(content) {
		t.Fatal("written file is not recognized as rmb opencode plugin")
	}
}

func TestPreviewOpenCode_pluginUsesReplaceDisplay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rmb"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	def, ok := agentDefByID(AgentOpenCode)
	if !ok {
		t.Fatal("opencode agent def missing")
	}
	state, err := previewOpenCode(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Artifacts) < 1 {
		t.Fatal("expected plugin artifact")
	}
	if state.Artifacts[0].DisplayMode != DisplayReplace {
		t.Fatalf("plugin display_mode = %q, want replace", state.Artifacts[0].DisplayMode)
	}
	if state.Artifacts[1].DisplayMode != DisplayDiff {
		t.Fatalf("recall display_mode = %q, want diff", state.Artifacts[1].DisplayMode)
	}
}
