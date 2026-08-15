package appshell

import (
	_ "embed"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"fyne.io/systray"
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
		<-sig
		daemon.Shutdown()
		systray.Quit()
	}()

	systray.Run(
		func() { onTrayReady(daemon) },
		func() { daemon.Shutdown() },
	)
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
		quit:   systray.AddMenuItem("Quit RMB", "Shut down RMB and quit"),
	}
	systray.AddSeparator()
	ui.status.Disable()
	ui.open.Disable()

	go ui.watchMenu()
	go ui.startup()
}

// startup ports the Tauri setup callback: bootstrap, recycle daemon after
// sidecar refresh, then hand over to the health poller.
func (ui *trayUI) startup() {
	if err := EnsureInstalled(); err != nil {
		stderrPrintf("bootstrap: %v\n", err)
	}
	// Always recycle rmbd after (re)installing sidecars so an old process
	// left behind by Quit / a previous version cannot stick.
	if err := ui.daemon.RestartAfterUpdate(); err != nil {
		stderrPrintf("start rmbd: %v\n", err)
	}
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
	if healthy {
		ui.status.SetTitle("🟢 RMB is running")
	} else {
		ui.status.SetTitle("Starting…")
	}
	if healthy {
		ui.status.Enable()
		ui.open.Enable()
	} else {
		ui.status.Disable()
		ui.open.Disable()
	}
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
