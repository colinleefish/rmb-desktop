// Package appshell is the rmb-desktop tray shell: it supervises the rmbd
// daemon, bootstraps sidecar binaries into ~/.rmb/bin, and renders the menu
// bar tray. Port of the former Tauri/Rust shell (app/src-tauri), Phase 1 of
// plan/tauri-to-go-shell.md — behavior-identical.
package appshell

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

// DefaultAddr mirrors the Rust shell's DEFAULT_ADDR.
const DefaultAddr = "127.0.0.1:19019"

// NormalizeAddr expands a bare host:port into an http(s) base URL, matching
// daemon.rs normalize_addr.
func NormalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "http://" + DefaultAddr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/")
	}
	return "http://" + strings.TrimRight(addr, "/")
}

// BaseURL resolves the daemon base URL: RMB_ADDR env, then config.yaml addr,
// then the default. Port of daemon.rs base_url.
func BaseURL() string {
	if v := os.Getenv("RMB_ADDR"); strings.TrimSpace(v) != "" {
		return NormalizeAddr(v)
	}
	if addr := readConfigAddr(); addr != "" {
		return NormalizeAddr(addr)
	}
	return NormalizeAddr(DefaultAddr)
}

// DaemonPort extracts the TCP port from BaseURL. Port of daemon.rs daemon_port.
func DaemonPort() uint16 {
	url := BaseURL()
	hostPort := strings.TrimPrefix(url, "https://")
	hostPort = strings.TrimPrefix(hostPort, "http://")
	if i := strings.LastIndex(hostPort, ":"); i >= 0 {
		if p, err := strconv.ParseUint(hostPort[i+1:], 10, 16); err == nil {
			return uint16(p)
		}
	}
	if strings.HasPrefix(url, "https://") {
		return 443
	}
	return 80
}

// DashboardURL is the web UI served by rmbd.
func DashboardURL() string {
	return strings.TrimRight(BaseURL(), "/") + "/ui/"
}

func readConfigAddr() string {
	path, err := platform.ConfigPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		Addr string `yaml:"addr"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Addr)
}

// HealthOK reports whether rmbd answers /healthz. Port of daemon.rs health_ok.
func HealthOK(base string) bool {
	url := strings.TrimRight(base, "/") + "/healthz"
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
