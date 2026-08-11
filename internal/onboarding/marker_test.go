package onboarding_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/onboarding"
	"github.com/colinleefish/rmb-desktop/internal/platform"
)

func isolateDataDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if os.Getenv("APPDATA") != "" {
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}
	if os.Getenv("XDG_DATA_HOME") != "" {
		t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	}
}

func TestMarkerRoundTrip(t *testing.T) {
	isolateDataDir(t)

	completed, _, path, err := onboarding.Status()
	if err != nil {
		t.Fatal(err)
	}
	if completed {
		t.Fatal("expected onboarding not completed by default")
	}

	markerPath, err := onboarding.MarkComplete(true)
	if err != nil {
		t.Fatal(err)
	}
	if markerPath != path {
		t.Fatalf("path=%s markerPath=%s", path, markerPath)
	}

	completed, marker, _, err := onboarding.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !completed || !marker.SkippedAgents || marker.CompletedAt == "" {
		t.Fatalf("marker=%+v completed=%v", marker, completed)
	}

	if err := onboarding.Reset(); err != nil {
		t.Fatal(err)
	}
	completed, _, _, err = onboarding.Status()
	if err != nil {
		t.Fatal(err)
	}
	if completed {
		t.Fatal("expected marker removed")
	}
}

func TestMarkerEmptyFileCountsAsComplete(t *testing.T) {
	isolateDataDir(t)

	path, err := platform.OnboardingCompletePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	completed, _, _, err := onboarding.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !completed {
		t.Fatal("expected empty marker file to count as complete")
	}
}
