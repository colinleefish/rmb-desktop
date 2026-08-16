package appshell

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/colinleefish/rmb-desktop/internal/platform"
	"github.com/colinleefish/rmb-desktop/internal/update"
	"github.com/colinleefish/rmb-desktop/internal/version"
)

// checkInterval is the background update-check period (24h); the first check
// runs shortly after startup so the tray settles first.
const (
	checkInterval = 24 * time.Hour
	firstCheckIn  = 15 * time.Second
)

// updateFeeds resolves the feed list for this install: env override →
// config.yaml update.mirrors → defaults (R2, GitHub).
func updateFeeds() []string {
	return update.Feeds(readUpdateMirrors())
}

func readUpdateMirrors() []string {
	path, err := platform.ConfigPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Update struct {
			Mirrors []string `yaml:"mirrors"`
		} `yaml:"update"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg.Update.Mirrors
}

// updaterCanRun: dev builds (no linked commit) never self-update.
func updaterCanRun() bool {
	return isKnownCommit(version.Commit)
}

// checkForUpdate wraps update.Check with this install's feeds and version.
func checkForUpdate(ctx context.Context) (*update.Release, error) {
	return update.Check(ctx, updateFeeds(), version.Version)
}

// CheckForUpdate is the exported headless entry (cmd/rmb-app -check-update).
func CheckForUpdate(ctx context.Context) (*update.Release, error) {
	return checkForUpdate(ctx)
}

// InstallUpdate is the exported headless entry (cmd/rmb-app -install-update).
func InstallUpdate(daemon *DaemonManager, rel *update.Release, onStage func(string)) error {
	return installUpdate(daemon, rel, onStage)
}

// installUpdate applies a release: stop the daemon (files may be locked on
// Windows), swap sidecars, restart. StopForUpdate (not Shutdown) is
// essential: Shutdown latches the quit flag and the restart below would be
// a silent no-op. On swap failure the updater rolls files back and we
// restart the previous daemon.
func installUpdate(daemon *DaemonManager, rel *update.Release, onStage func(string)) error {
	daemon.StopForUpdate()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := update.Apply(ctx, rel, installDir(), onStage); err != nil {
		// Rollback restored the old files — bring the old daemon back.
		if rbErr := daemon.RestartAfterUpdate(); rbErr != nil {
			stderrPrintf("update rollback restart: %v\n", rbErr)
		}
		return err
	}
	return daemon.RestartAfterUpdate()
}

func installDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".rmb", "bin")
}
