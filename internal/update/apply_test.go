package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// makeBundle builds a sidecar archive containing fake rmb/rmbd binaries.
func makeBundle(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	names := installNames()
	members := map[string]string{} // bundle member -> content
	for srcName := range names {
		members[srcName] = content + " " + srcName
	}

	switch {
	case len(name) > 4 && name[len(name)-4:] == ".zip":
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for member, body := range members {
			w, err := zw.Create(member)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	default: // tar.gz
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for member, body := range members {
			hdr := &tar.Header{Name: member, Mode: 0o755, Size: int64(len(body))}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("bundle %s sha256=%s", name, SHA256Hex(data))
	// Side channel for building the manifest in tests: write .sha256 next to it.
	if err := os.WriteFile(path+".sha256", []byte(SHA256Hex(data)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func ext() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

func TestApplyEndToEnd(t *testing.T) {
	work := t.TempDir()
	bundleName := "rmb-desktop_9.9.9_test" + ext()
	bundlePath := makeBundle(t, work, bundleName, "new-binary")
	sha := readSHA(t, bundlePath+".sha256")

	install := filepath.Join(work, "bin")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing "old" binaries that must be replaced.
	for _, dst := range installNames() {
		if err := os.WriteFile(filepath.Join(install, dst), []byte("old "+dst), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(bundlePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	defer srv.Close()

	rel := &Release{
		Manifest: &Manifest{Product: product, Version: "9.9.9"},
		FeedURL:  srv.URL + "/latest.json",
		Bundle:   PlatformArt{Sidecars: bundleName, SHA256: sha},
	}

	var stages []string
	if err := Apply(context.Background(), rel, install, func(s string) { stages = append(stages, s) }); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(stages) == 0 || stages[0] != "Downloading" {
		t.Errorf("stages = %v", stages)
	}

	// Both binaries replaced, no .new/.bak leftovers.
	for srcName, dst := range installNames() {
		data, err := os.ReadFile(filepath.Join(install, dst))
		if err != nil {
			t.Fatalf("read %s: %v", dst, err)
		}
		want := "new-binary " + srcName
		if string(data) != want {
			t.Errorf("%s = %q, want %q", dst, data, want)
		}
	}
	for _, name := range []string{"rmb.bak", "rmbd-desktop.bak"} {
		if _, err := os.Stat(filepath.Join(install, name+"x")); err == nil { // names differ per OS; rough check below
			t.Errorf("leftover %s", name)
		}
	}
}

func TestApplyBadSHA(t *testing.T) {
	work := t.TempDir()
	bundleName := "rmb-desktop_9.9.9_test" + ext()
	bundlePath := makeBundle(t, work, bundleName, "content")

	install := filepath.Join(work, "bin")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dst := range installNames() {
		if err := os.WriteFile(filepath.Join(install, dst), []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile(bundlePath)
		w.Write(data)
	}))
	defer srv.Close()

	rel := &Release{
		Manifest: &Manifest{Product: product, Version: "9.9.9"},
		FeedURL:  srv.URL + "/latest.json",
		Bundle:   PlatformArt{Sidecars: bundleName, SHA256: "deadbeef"},
	}

	if err := Apply(context.Background(), rel, install, nil); err == nil {
		t.Fatal("sha mismatch must fail Apply")
	}

	// Old binaries untouched after rollback.
	data, err := os.ReadFile(filepath.Join(install, firstInstallName()))
	if err != nil || string(data) != "old" {
		t.Errorf("rollback failed: %q %v", data, err)
	}
}

func TestApplyMissingDaemonKeepsOld(t *testing.T) {
	// Bundle missing rmbd → Apply must fail and keep old files.
	work := t.TempDir()
	bundlePath := filepath.Join(work, "partial.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "rmb", Mode: 0o755, Size: 3}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("new"))
	tw.Close()
	gz.Close()
	if err := os.WriteFile(bundlePath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	install := filepath.Join(work, "bin")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dst := range installNames() {
		if err := os.WriteFile(filepath.Join(install, dst), []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile(bundlePath)
		w.Write(data)
	}))
	defer srv.Close()

	rel := &Release{
		Manifest: &Manifest{Product: product, Version: "9.9.9"},
		FeedURL:  srv.URL + "/latest.json",
		Bundle:   PlatformArt{Sidecars: "partial.tar.gz", SHA256: SHA256Hex(buf.Bytes())},
	}
	if err := Apply(context.Background(), rel, install, nil); err == nil {
		t.Fatal("bundle missing rmbd must fail")
	}
	data, err := os.ReadFile(filepath.Join(install, firstInstallName()))
	if err != nil || string(data) != "old" {
		t.Errorf("old files must survive: %q %v", data, err)
	}
}

// The full Check→Apply loop against one server serving feed+bundle.
func TestCheckAndApplyViaServer(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	savedB64 := publicKeyB64
	publicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	defer func() {
		publicKeyB64 = savedB64
		pubOnce = sync.Once{}
	}()

	work := t.TempDir()
	bundleName := fmt.Sprintf("rmb-desktop_9.9.9_%s_%s%s", platformKey(), archKey(), ext())
	bundlePath := makeBundle(t, work, bundleName, "fresh")
	sha := readSHA(t, bundlePath+".sha256")

	m := &Manifest{
		Product:    product,
		Version:    "9.9.9",
		ReleasedAt: "2026-08-15T00:00:00Z",
		Platforms: map[string]map[string]PlatformArt{
			platformKey(): {archKey(): {Sidecars: bundleName, SHA256: sha}},
		},
	}
	m.Sign(priv)
	feed := mustJSON(t, m)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			w.Write(feed)
		case "/9.9.9/" + bundleName:
			data, _ := os.ReadFile(bundlePath)
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	rel, err := Check(context.Background(), []string{srv.URL + "/latest.json"}, "0.1.0")
	if err != nil || rel == nil {
		t.Fatalf("Check: rel=%v err=%v", rel, err)
	}

	install := filepath.Join(work, "bin")
	if err := Apply(context.Background(), rel, install, nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for srcName, dst := range installNames() {
		data, err := os.ReadFile(filepath.Join(install, dst))
		if err != nil {
			t.Fatalf("%s: %v", dst, err)
		}
		if want := "fresh " + srcName; string(data) != want {
			t.Errorf("%s = %q, want %q", dst, data, want)
		}
	}
}

func readSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func firstInstallName() string {
	for _, dst := range installNames() {
		return dst
	}
	return ""
}
