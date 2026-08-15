// Command rmb-app is the rmb-desktop menu bar shell: tray UI, rmbd daemon
// supervision, and sidecar bootstrap. Replaces the Tauri shell as of 0.2.0
// (plan/tauri-to-go-shell.md).
package main

import (
	"os"

	"github.com/colinleefish/rmb-desktop/internal/appshell"
)

func main() {
	release, err := appshell.AcquireInstanceLock()
	if err != nil {
		// Already running — exit silently, like the Tauri shell did.
		os.Exit(0)
	}
	defer release()

	appshell.Run()
}
