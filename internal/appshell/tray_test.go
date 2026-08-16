package appshell

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusTooltip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := statusTooltip(true, ""); !strings.HasPrefix(got, "RMB status: running") {
		t.Errorf("healthy tooltip: %q", got)
	}

	got := statusTooltip(false, "start rmbd at /x/rmbd-desktop: exec format error")
	if !strings.HasPrefix(got, "RMB status: start rmbd") {
		t.Errorf("error tooltip prefix: %q", got)
	}
	if !strings.Contains(got, "exec format error") {
		t.Errorf("error tooltip must carry the reason: %q", got)
	}

	// Detail is truncated so a pathological error can't make the tooltip
	// unreadable, but the log path always survives on its own line.
	got = statusTooltip(false, strings.Repeat("e", 500))
	if strings.Count(got, "e") > 210 {
		t.Errorf("error must be truncated in tooltip")
	}
	if want := filepath.Join("logs", "rmbd.log"); !strings.Contains(got, want) {
		t.Errorf("log path must survive truncation: %q", got)
	}

	if got := statusTooltip(false, ""); !strings.HasPrefix(got, "RMB status: starting") {
		t.Errorf("starting tooltip: %q", got)
	}
}
