//go:build darwin

package launchatlogin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const label = "me.remember.rmb"

func set(enabled bool) error {
	if enabled {
		return enable()
	}
	return disable()
}

func enable() error {
	plistPath, err := plistPath()
	if err != nil {
		return err
	}
	if err := writePlist(plistPath); err != nil {
		return err
	}

	uid, err := guiUID()
	if err != nil {
		return err
	}
	domain := fmt.Sprintf("gui/%s", uid)
	target := fmt.Sprintf("%s/%s", domain, label)

	bootout(domain, plistPath)

	if err := runLaunchctl("bootstrap", domain, plistPath); err != nil {
		return err
	}
	return runLaunchctl("enable", target)
}

func disable() error {
	uid, err := guiUID()
	if err != nil {
		return err
	}
	domain := fmt.Sprintf("gui/%s", uid)
	target := fmt.Sprintf("%s/%s", domain, label)

	_ = runLaunchctl("disable", target)
	plistPath, err := plistPath()
	if err != nil {
		return err
	}
	bootout(domain, plistPath)
	return nil
}

func bootout(domain, plistPath string) {
	_ = runLaunchctl("bootout", domain, plistPath)
	_ = runLaunchctl("bootout", domain, label)
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

func appBinaryPath(home string) string {
	app := filepath.Join(home, ".rmb", "bin", "rmb-app")
	if fileExists(app) {
		return app
	}
	return filepath.Join(home, ".rmb", "bin", "rmb")
}

func writePlist(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	appBin := appBinaryPath(home)
	daemonBin := filepath.Join(home, ".rmb", "bin", "rmbd-desktop")

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir LaunchAgents: %w", err)
	}

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>RMBD_PATH</key>
        <string>%s</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>/tmp/rmb-app.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/rmb-app.stderr.log</string>
</dict>
</plist>
`, label, appBin, daemonBin)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	return nil
}

func guiUID() (string, error) {
	out, err := exec.Command("id", "-u").Output()
	if err != nil {
		return "", fmt.Errorf("id -u: %w", err)
	}
	uid := strings.TrimSpace(string(out))
	if uid == "" {
		return "", fmt.Errorf("empty uid")
	}
	return uid, nil
}

func runLaunchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("launchctl %s: %w", strings.Join(args, " "), err)
		}
		return fmt.Errorf("launchctl %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
