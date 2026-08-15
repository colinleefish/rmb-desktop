package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Release is a verified, downloadable update for this runtime.
type Release struct {
	Manifest *Manifest
	FeedURL  string
	Bundle   PlatformArt
}

// BundleURL is the absolute download URL for the release bundle.
func (r *Release) BundleURL() string {
	return BundleURL(r.FeedURL, r.Bundle.Sidecars)
}

// Check fetches feeds in order and returns the first verified manifest that
// is newer than currentVersion and has a bundle for this runtime.
// Errors are per-feed; all feeds failing yields the last error.
func Check(ctx context.Context, feeds []string, currentVersion string) (*Release, error) {
	client := httpClient(15 * time.Second)
	var lastErr error
	for _, feed := range feeds {
		m, err := FetchManifest(ctx, client, feed)
		if err != nil {
			lastErr = err
			continue
		}
		if err := m.Verify(); err != nil {
			lastErr = err
			continue
		}
		newer, err := IsNewer(m.Version, currentVersion)
		if err != nil {
			lastErr = err
			continue
		}
		if !newer {
			return nil, nil // up to date — signature and version are valid
		}
		art, err := BundleFor(m)
		if err != nil {
			lastErr = err
			continue
		}
		return &Release{Manifest: m, FeedURL: feed, Bundle: art}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// installNames maps bundle members to their installed names in ~/.rmb/bin.
func installNames() map[string]string {
	if runtime.GOOS == "windows" {
		return map[string]string{"rmb.exe": "rmb.exe", "rmbd.exe": "rmbd-desktop.exe"}
	}
	return map[string]string{"rmb": "rmb", "rmbd": "rmbd-desktop"}
}

// Apply downloads and verifies the bundle, then swaps the sidecars into
// installDir. The daemon must already be stopped (caller's job). On any
// failure the previous files are restored.
func Apply(ctx context.Context, rel *Release, installDir string, onStage func(string)) error {
	if onStage == nil {
		onStage = func(string) {}
	}
	tmp, err := os.MkdirTemp("", "rmb-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	onStage("Downloading")
	bundlePath := filepath.Join(tmp, "bundle")
	if err := download(ctx, rel.BundleURL(), bundlePath, rel.Bundle.SHA256); err != nil {
		return err
	}

	onStage("Verifying")
	if err := verifyFile(bundlePath, strings.ToLower(rel.Bundle.SHA256)); err != nil {
		return err
	}

	onStage("Extracting")
	extractDir := filepath.Join(tmp, "x")
	if err := extract(bundlePath, extractDir); err != nil {
		return err
	}

	onStage("Installing")
	return swapAll(extractDir, installDir)
}

func download(ctx context.Context, url, dst, wantSHA string) error {
	client := httpClient(10 * time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("update: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download %s: status %d", url, resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return fmt.Errorf("update: download %s: %w", url, err)
	}
	if got := SHA256Hex(buf.Bytes()); got != strings.ToLower(wantSHA) {
		return fmt.Errorf("update: bundle sha256 mismatch: got %s want %s", got, wantSHA)
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

func verifyFile(path, wantSHA string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if got := SHA256Hex(data); got != strings.ToLower(wantSHA) {
		return fmt.Errorf("update: bundle sha256 mismatch: got %s want %s", got, wantSHA)
	}
	return nil
}

// extract unpacks tar.gz or zip bundles into dir.
func extract(bundlePath, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(bundlePath, ".zip") {
		return extractZip(bundlePath, dir)
	}
	return extractTargz(bundlePath, dir)
}

func extractTargz(bundlePath, dir string) error {
	f, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("update: not gzip: %w", err)
	}
	tr := tar.NewReader(gz)
	names := installNames()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("update: read bundle: %w", err)
		}
		base := filepath.Base(hdr.Name)
		dstName, ok := names[base]
		if hdr.Typeflag != tar.TypeReg || !ok {
			continue
		}
		if err := writeExecutable(tr, filepath.Join(dir, dstName)); err != nil {
			return err
		}
	}
	return requireExtracted(dir, names)
}

func extractZip(bundlePath, dir string) error {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("update: open zip: %w", err)
	}
	defer zr.Close()
	names := installNames()
	for _, zf := range zr.File {
		dstName, ok := names[filepath.Base(zf.Name)]
		if zf.FileInfo().IsDir() || !ok {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeExecutable(rc, filepath.Join(dir, dstName))
		rc.Close()
		if err != nil {
			return err
		}
	}
	return requireExtracted(dir, names)
}

func writeExecutable(r io.Reader, dst string) error {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func requireExtracted(dir string, names map[string]string) error {
	for _, dst := range names {
		if _, err := os.Stat(filepath.Join(dir, dst)); err != nil {
			return fmt.Errorf("update: bundle is missing %q", dst)
		}
	}
	return nil
}

// swapAll replaces installDir entries with extracted ones, rolling back on
// the first failure. .new staging keeps the window small; on Windows the
// final rename relies on the daemon being stopped (no open handles).
func swapAll(extractDir, installDir string) error {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	names := installNames()

	var installed []string // install names done, for rollback
	rollback := func(cause error) error {
		for _, name := range installed {
			bak := filepath.Join(installDir, name+".bak")
			if _, err := os.Stat(bak); err == nil {
				_ = os.Rename(bak, filepath.Join(installDir, name))
			} else {
				_ = os.Remove(filepath.Join(installDir, name))
			}
		}
		return cause
	}

	for _, dstName := range names {
		src := filepath.Join(extractDir, dstName)
		dst := filepath.Join(installDir, dstName)
		bak := dst + ".bak"

		if err := copyMode(src, dst+".new"); err != nil {
			return rollback(err)
		}
		if _, err := os.Stat(dst); err == nil {
			if err := os.Rename(dst, bak); err != nil {
				return rollback(err)
			}
		}
		if err := os.Rename(dst+".new", dst); err != nil {
			return rollback(err)
		}
		installed = append(installed, dstName)
	}

	// Success: drop rollback backups.
	for _, name := range names {
		_ = os.Remove(filepath.Join(installDir, name+".bak"))
	}
	return nil
}

func copyMode(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ErrInProgress sentinel kept for future concurrent-install guards.
var ErrInProgress = errors.New("update: another update is in progress")
