//go:build darwin

package appshell

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/launchatlogin"
)

// loginItemPollSeconds is how often the coordinator re-reads the daemon
// config to pick up launch_at_login toggles made in the webui while the
// app is running.
const loginItemPollSeconds = 10

// RunLoginItemCoordinator keeps the macOS login-item registration in sync
// with the daemon config's launch_at_login flag. It must run inside the
// app bundle process: SMAppService.mainApp only exists there. rmbd stores
// the flag (config.yaml is the single source of truth); this coordinator
// applies it.
//
// Behaviour:
//   - first pass: migrate any legacy LaunchAgent away (one-shot, retried on
//     transient failure), then apply the desired state;
//   - subsequent passes: re-apply only when the config value changed
//     (edge-triggered, so a manual removal in System Settings is not
//     fought while the config still says enabled... it is re-applied only
//     on an actual toggle).
//
// The initial sleep waits for the tray bootstrap, which recycles rmbd.
func RunLoginItemCoordinator() {
	sleep(3)
	c := &loginItemCoordinator{
		fetch:   fetchLaunchAtLogin,
		set:     launchatlogin.SetFromBundle,
		migrate: launchatlogin.MigrateFromLegacy,
		logf: func(format string, args ...any) {
			stderrPrintf("login-item: "+format+"\n", args...)
		},
	}
	for {
		c.tick()
		sleep(loginItemPollSeconds)
	}
}

// loginItemCoordinator is the testable core of RunLoginItemCoordinator.
type loginItemCoordinator struct {
	fetch   func() (bool, error)
	set     func(bool) error
	migrate func() (bool, error)
	logf    func(format string, args ...any)

	migrated bool
	applied  *bool
}

// tick runs one reconcile pass. Failures are logged and retried on the next
// tick (desired state is never lost — config.yaml remains the source of
// truth).
func (c *loginItemCoordinator) tick() {
	desired, err := c.fetch()
	if err != nil {
		c.logf("read config: %v", err)
		return
	}
	if !c.migrated {
		hadLegacy, err := c.migrate()
		if err != nil {
			c.logf("legacy migration: %v", err)
			return
		}
		c.migrated = true
		if hadLegacy {
			c.logf("migrated legacy LaunchAgent to SMAppService")
		}
	}
	if c.applied == nil || *c.applied != desired {
		if err := c.set(desired); err != nil {
			c.logf("apply %v: %v", desired, err)
			return
		}
		c.applied = &desired
		if desired {
			c.logf("login item enabled (RMB Desktop)")
		} else {
			c.logf("login item disabled")
		}
	}
}

// fetchLaunchAtLogin reads launch_at_login from the daemon config API.
func fetchLaunchAtLogin() (bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(BaseURL() + "/api/v1/config")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GET /api/v1/config: %s", resp.Status)
	}
	var view struct {
		LaunchAtLogin bool `json:"launch_at_login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		return false, err
	}
	return view.LaunchAtLogin, nil
}
