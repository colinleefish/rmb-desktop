package appshell

import (
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/update"
	"github.com/colinleefish/rmb-desktop/internal/version"
)

// B03: tray update check must not compare the feed only against the shell's
// linked version when sidecars were updated in place.
func TestB03_updateCheckVersionUsesInstalledSidecar(t *testing.T) {
	dir := t.TempDir()
	const installed = "99.99.99"
	if err := update.WriteSidecarStamp(dir, installed); err != nil {
		t.Fatal(err)
	}
	got := updateCheckVersion(dir)
	if got == installed {
		return
	}
	t.Fatalf("update baseline %q want %q (shell linked %q); checkForUpdate compares feed against shell version, not installed sidecars",
		got, installed, version.Version)
}
