package appshell

import (
	_ "embed"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"fyne.io/systray"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

//go:embed assets/tray-icon.png
var trayIcon []byte

// Run starts the tray shell and blocks until Quit. Port of main.rs.
func Run() {
	daemon := NewDaemonManager()

	// Deviation from the Rust shell: trap SIGINT/SIGTERM so `kill` (and
	// launchd on logout) shuts the daemon down cleanly instead of orphaning it.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig
		stderrPrintf("DEBUG: signal %v received, shutting down\n", s)
		daemon.Shutdown()
		systray.Quit()
	}()

	systray.Run(
		func() {
			stderrPrintf("DEBUG: onReady\n")
			onTrayReady(daemon)
		},
		func() {
			stderrPrintf("DEBUG: onExit\n")
			daemon.Shutdown()
		},
	)
	stderrPrintf("DEBUG: systray.Run returned\n")
}

type trayUI struct {
	daemon *DaemonManager
	status *systray.MenuItem
	open   *systray.MenuItem
	quit   *systray.MenuItem
}

func onTrayReady(daemon *DaemonManager) {
	systray.SetTemplateIcon(trayIcon, trayIcon)
	systray.SetTooltip("RMB Desktop")

	ui := &trayUI{
		daemon: daemon,
		status: systray.AddMenuItem("Starting…", "RMB status"),
		open:   systray.AddMenuItem("Open Dashboard", "Open the RMB web dashboard"),
	}
	systray.AddSeparator()
	updater := newUpdaterUI(daemon)
	ui.quit = systray.AddMenuItem("Quit RMB", "Shut down RMB and quit")
	ui.status.Disable()
	ui.open.Disable()

	go ui.watchMenu()
	go updater.watch()
	go updater.background()
	go ui.startup()
}

// startup ports the Tauri setup callback: bootstrap, recycle daemon after
// sidecar refresh, then hand over to the health poller.
func (ui *trayUI) startup() {
	stderrPrintf("DEBUG: startup begin\n")
	if err := EnsureInstalled(); err != nil {
		stderrPrintf("bootstrap: %v\n", err)
	}
	stderrPrintf("DEBUG: bootstrap done\n")
	// Always recycle rmbd after (re)installing sidecars so an old process
	// left behind by Quit / a previous version cannot stick.
	if err := ui.daemon.RestartAfterUpdate(); err != nil {
		stderrPrintf("start rmbd: %v\n", err)
	}
	stderrPrintf("DEBUG: restart done\n")
	ui.refreshMenu()
	ui.healthPoller()
}

// healthPoller ports spawn_health_poller: recover a dead daemon and refresh
// the menu every 5 seconds.
func (ui *trayUI) healthPoller() {
	for {
		sleep(5)
		if !HealthOK(BaseURL()) {
			if err := ui.daemon.EnsureRunning(); err != nil {
				// Expected once Quit has begun shutting the daemon down.
				if err.Error() != "shutting down" {
					stderrPrintf("ensure rmbd: %v\n", err)
				}
			}
		}
		ui.refreshMenu()
	}
}

func (ui *trayUI) refreshMenu() {
	healthy := HealthOK(BaseURL())
	lastErr := ui.daemon.LastError()
	if healthy {
		ui.status.SetTitle("🟢 RMB is running")
	} else if lastErr != "" {
		// Keep the title a fixed short phrase — the menu bar is always
		// visible and a long error string would be unreadable noise.
		ui.status.SetTitle("🔴 RMB failed to start")
	} else {
		ui.status.SetTitle("Starting…")
	}
	ui.status.SetTooltip(statusTooltip(healthy, lastErr))
	if healthy {
		ui.status.Enable()
		ui.open.Enable()
	} else {
		ui.status.Disable()
		ui.open.Disable()
	}
}

// statusTooltip carries the diagnostics the title deliberately omits:
// the failure reason (hover to read it) and the daemon log path.
func statusTooltip(healthy bool, lastErr string) string {
	var parts []string
	if healthy {
		parts = append(parts, "RMB status: running")
	} else if lastErr != "" {
		parts = append(parts, "RMB status: "+truncate(lastErr, 200))
	} else {
		parts = append(parts, "RMB status: starting")
	}
	if path, err := platform.DaemonLogPath(); err == nil {
		parts = append(parts, "daemon log: "+path)
	}
	return strings.Join(parts, "\n")
}

func (ui *trayUI) watchMenu() {
	for {
		select {
		case <-ui.open.ClickedCh:
			if !HealthOK(BaseURL()) {
				continue
			}
			if err := openURL(DashboardURL()); err != nil {
				stderrPrintf("open dashboard: %v\n", err)
			}
		case <-ui.quit.ClickedCh:
			ui.daemon.Shutdown()
			systray.Quit()
			return
		}
	}
}

// openURL replaces the Rust `open` crate: open a URL in the default browser.
func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
