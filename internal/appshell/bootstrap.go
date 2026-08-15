package appshell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	installDirName = ".rmb/bin"
	cliName        = "rmb"
	appCliName     = "rmb-app"
	daemonName     = "rmbd-desktop"
)

// EnsureInstalled copies the bundled rmb/rmbd sidecars and the app itself
// into ~/.rmb/bin, refreshing when the bundled sidecars are newer. Port of
// bootstrap.rs ensure_installed.
func EnsureInstalled() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}
	installDir := filepath.Join(home, ".rmb", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", installDir, err)
	}

	cliDst := filepath.Join(installDir, cliName)
	daemonDst := filepath.Join(installDir, daemonName)
	appDst := filepath.Join(installDir, appCliName)

	refresh, err := needsRefresh(cliDst, daemonDst)
	if err != nil {
		return err
	}
	if refresh {
		if err := installSidecar("rmb", cliDst); err != nil {
			return err
		}
		if err := installSidecar("rmbd", daemonDst); err != nil {
			return err
		}
		if err := installCurrentExe(appDst); err != nil {
			return err
		}
	} else if !isFile(appDst) {
		if err := installCurrentExe(appDst); err != nil {
			return err
		}
	}
	return nil
}

// InstalledDaemonPath is the daemon location used by launch-at-login and as
// a find_rmbd_binary fallback. Port of installed_daemon_path.
func InstalledDaemonPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, installDirName, daemonName)
}

// needsRefresh ports needs_refresh/is_newer: refresh when either destination
// is missing or its bundled source has a newer mtime.
func needsRefresh(cliDst, daemonDst string) (bool, error) {
	if !isFile(cliDst) || !isFile(daemonDst) {
		return true, nil
	}
	bundledRmb, err := bundledSidecarPath("rmb")
	if err != nil {
		return false, err
	}
	bundledRmbd, err := bundledSidecarPath("rmbd")
	if err != nil {
		return false, err
	}
	rmbNewer, err := isNewer(bundledRmb, cliDst)
	if err != nil {
		return false, err
	}
	rmbdNewer, err := isNewer(bundledRmbd, daemonDst)
	if err != nil {
		return false, err
	}
	return rmbNewer || rmbdNewer, nil
}

func isNewer(src, dst string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", src, err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	return srcInfo.ModTime().After(dstInfo.ModTime()), nil
}

func installCurrentExe(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("current_exe: %w", err)
	}
	return copyExecutable(exe, dst)
}

func installSidecar(baseName, dst string) error {
	src, err := bundledSidecarPath(baseName)
	if err != nil {
		return err
	}
	return copyExecutable(src, dst)
}

func copyExecutable(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dst, 0o755); err != nil {
			return fmt.Errorf("chmod %s: %w", dst, err)
		}
	}
	return nil
}

// bundledSidecarPath ports bundled_sidecar_path: the sidecar next to the
// running exe, any target-triple-prefixed sibling, or repo bin/ dev fallback.
func bundledSidecarPath(baseName string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("current_exe: %w", err)
	}
	exeDir := filepath.Dir(exe)

	candidates := []string{filepath.Join(exeDir, baseName)}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(exeDir, baseName+".exe"))
	}
	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	if found := findPrefixedBinary(exeDir, baseName); found != "" {
		return found, nil
	}

	// Dev fallback: repo bin/ when running from target/debug or target/release.
	for _, devBin := range []string{"../../../bin", "../../../../bin"} {
		p := filepath.Join(exeDir, devBin, baseName)
		if isFile(p) {
			if abs, err := filepath.Abs(p); err == nil {
				return abs, nil
			}
			return p, nil
		}
	}

	return "", fmt.Errorf("bundled sidecar %s not found near %s", baseName, exe)
}

// findPrefixedBinary ports find_prefixed_binary: <base> or <base>-<triple>.
func findPrefixedBinary(dir, baseName string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	prefix := baseName + "-"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == baseName || (len(name) > len(prefix) && name[:len(prefix)] == prefix) {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
