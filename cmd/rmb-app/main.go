// Command rmb-app is the rmb-desktop menu bar shell: tray UI, rmbd daemon
// supervision, sidecar bootstrap, and the sidecar self-updater. Replaces the
// Tauri shell as of 0.2.0 (plan/tauri-to-go-shell.md).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/colinleefish/rmb-desktop/internal/appshell"
)

func main() {
	checkUpdate := flag.Bool("check-update", false, "check feeds for an update, print result, exit")
	installUpdate := flag.Bool("install-update", false, "check and install an update headlessly, exit")
	flag.Parse()

	if *checkUpdate || *installUpdate {
		os.Exit(runHeadlessUpdate(*installUpdate))
	}

	release, err := appshell.AcquireInstanceLock()
	if err != nil {
		// Already running — exit silently, like the Tauri shell did.
		os.Exit(0)
	}
	defer release()

	// Login-item reconcile must run inside the bundle process (SMAppService).
	go appshell.RunLoginItemCoordinator()
	appshell.Run()
}

// runHeadlessUpdate drives the updater without the tray (CI/smoke tests).
func runHeadlessUpdate(install bool) int {
	rel, err := appshell.CheckForUpdate(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check: %v\n", err)
		return 1
	}
	if rel == nil {
		fmt.Println("up to date")
		return 0
	}
	fmt.Printf("update available: v%s (%s)\n", rel.Manifest.Version, rel.BundleURL())
	if !install {
		return 0
	}
	daemon := appshell.NewDaemonManager()
	if err := appshell.InstallUpdate(daemon, rel, func(stage string) {
		fmt.Println("  ", stage)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "update install: %v\n", err)
		return 1
	}
	fmt.Printf("updated to v%s\n", rel.Manifest.Version)
	return 0
}
