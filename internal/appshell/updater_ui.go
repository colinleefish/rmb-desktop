package appshell

import (
	"context"
	"fmt"
	"time"

	"fyne.io/systray"
)

// checkTimeout bounds a feed check (several feeds × ~15s HTTP budget).
const checkTimeout = 45 * time.Second

// updaterUI owns the update menu items and drives checks/installs.
type updaterUI struct {
	daemon  *DaemonManager
	check   *systray.MenuItem
	install *systray.MenuItem
	title   string // last install-item title (systray has no getter)
	busy    bool
}

func newUpdaterUI(daemon *DaemonManager) *updaterUI {
	u := &updaterUI{
		daemon: daemon,
		check:  systray.AddMenuItem("Check for Updates…", "Check for a newer rmb-desktop"),
	}
	u.install = systray.AddMenuItem("", "Install the downloaded update")
	u.install.Hide()
	return u
}

func (u *updaterUI) setInstallTitle(title string) {
	u.title = title
	u.install.SetTitle(title)
}

// watch handles menu clicks for check/install.
func (u *updaterUI) watch() {
	for {
		select {
		case <-u.check.ClickedCh:
			u.runCheck(true)
		case <-u.install.ClickedCh:
			u.runInstall()
		}
	}
}

// background runs the periodic check (first shortly after launch, then daily).
func (u *updaterUI) background() {
	if !updaterCanRun() {
		u.check.SetTitle("Updates (dev build)")
		u.check.Disable()
		return
	}
	time.Sleep(firstCheckIn)
	u.runCheck(false)
	for {
		time.Sleep(checkInterval)
		u.runCheck(false)
	}
}

// runCheck queries the feeds; interactive=true reflects result in the menu.
func (u *updaterUI) runCheck(interactive bool) {
	if u.busy || !updaterCanRun() {
		return
	}
	u.busy = true
	defer func() { u.busy = false }()

	if interactive {
		u.check.SetTitle("Checking…")
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	rel, err := checkForUpdate(ctx)
	switch {
	case err != nil:
		stderrPrintf("update check: %v\n", err)
		if interactive {
			u.check.SetTitle("⚠ Check failed")
			time.Sleep(3 * time.Second)
		}
	case rel == nil:
		if interactive {
			u.check.SetTitle("✓ Up to date")
			time.Sleep(3 * time.Second)
		}
		u.install.Hide()
	default:
		u.setInstallTitle(fmt.Sprintf("🆕 v%s — Install Update", rel.Manifest.Version))
		u.install.Show()
	}
	if interactive {
		u.check.SetTitle("Check for Updates…")
	}
}

// runInstall performs the sidecar swap and daemon restart.
func (u *updaterUI) runInstall() {
	if u.busy || !updaterCanRun() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	rel, err := checkForUpdate(ctx)
	if err != nil || rel == nil {
		// Feed changed under us — drop the stale offer.
		stderrPrintf("update install: recheck failed: %v\n", err)
		u.install.Hide()
		return
	}

	u.busy = true
	defer func() { u.busy = false }()

	u.setInstallTitle("Updating…")
	u.check.Disable()
	err = installUpdate(u.daemon, rel, func(stage string) {
		u.setInstallTitle("Updating: " + stage + "…")
	})
	u.check.Enable()
	if err != nil {
		stderrPrintf("update install: %v\n", err)
		u.setInstallTitle("⚠ Update failed")
		time.Sleep(5 * time.Second)
		u.setInstallTitle(u.title) // keep the offer; user can retry
		return
	}
	u.setInstallTitle("✓ Updated")
	time.Sleep(5 * time.Second)
	u.install.Hide()
}
