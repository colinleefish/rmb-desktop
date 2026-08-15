// Package update implements the rmb-desktop self-updater for sidecar
// binaries (Phase 2 of plan/tauri-to-go-shell.md). The .app shell itself is
// not updated in place; the feed ships rmb/rmbd bundles which are swapped
// into ~/.rmb/bin, then the shell restarts the daemon.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Manifest v2. Served as R2 latest.json and as the manifest.json release
// asset on GitHub; mirrors may host either name.
type Manifest struct {
	Product     string                            `json:"product"`
	Version     string                            `json:"version"`
	ReleasedAt  string                            `json:"released_at"`
	Signature   string                            `json:"signature,omitempty"`
	Platforms   map[string]map[string]PlatformArt `json:"platforms"`
}

// PlatformArt describes one platform/arch sidecar bundle.
type PlatformArt struct {
	// Sidecars is the bundle file name, relative to <feed-dir>/<version>/.
	Sidecars string `json:"sidecars"`
	SHA256   string `json:"sha256"`
}

const product = "rmb-desktop"

// ErrNoSignature is returned when a manifest lacks a signature — updates are
// refused rather than trusted on sha256 alone.
var ErrNoSignature = errors.New("update: manifest is not signed")

// ErrBadSignature is returned when the manifest signature does not verify
// against the built-in public key (possible mirror tampering).
var ErrBadSignature = errors.New("update: manifest signature verification failed")

// FetchManifest downloads and verifies a manifest from one feed URL.
func FetchManifest(ctx context.Context, client *http.Client, feedURL string) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: feed %s: status %d", feedURL, resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return nil, fmt.Errorf("update: feed %s: %w", feedURL, err)
	}
	if m.Product != product {
		return nil, fmt.Errorf("update: feed %s: product %q", feedURL, m.Product)
	}
	return &m, nil
}

// Verify checks the manifest signature against the embedded public key.
func (m *Manifest) Verify() error {
	if strings.TrimSpace(m.Signature) == "" {
		return ErrNoSignature
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("%w: signature is not valid base64", ErrBadSignature)
	}
	pub, err := publicKey()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, m.Canonical(), sig) {
		return ErrBadSignature
	}
	return nil
}

// Canonical returns the deterministic byte string covered by the signature:
// product, version, released_at, then every platform/arch entry in sorted
// order. JSON key order and formatting never affect the signed bytes.
func (m *Manifest) Canonical() []byte {
	var b strings.Builder
	b.WriteString(product)
	b.WriteByte('\n')
	b.WriteString(m.Version)
	b.WriteByte('\n')
	b.WriteString(m.ReleasedAt)
	b.WriteByte('\n')

	platforms := make([]string, 0, len(m.Platforms))
	for p := range m.Platforms {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)
	for _, p := range platforms {
		archs := make([]string, 0, len(m.Platforms[p]))
		for a := range m.Platforms[p] {
			archs = append(archs, a)
		}
		sort.Strings(archs)
		for _, a := range archs {
			art := m.Platforms[p][a]
			b.WriteString(p)
			b.WriteByte('/')
			b.WriteString(a)
			b.WriteByte('\n')
			b.WriteString(art.Sidecars)
			b.WriteByte('\n')
			b.WriteString(strings.ToLower(art.SHA256))
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

// Sign injects an ed25519 signature using the given private key (test and
// tooling path; production manifests are signed by cmd/manifest-sign).
func (m *Manifest) Sign(priv ed25519.PrivateKey) {
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, m.Canonical()))
}

// VerifyWith is Verify against an explicit key (tests, manifest-sign).
func (m *Manifest) VerifyWith(pub ed25519.PublicKey) bool {
	if m.Signature == "" {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, m.Canonical(), sig)
}

// SHA256Hex hashes data as lowercase hex (manifest convention).
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// httpClient builds a client with the given timeout for feed/download use.
func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
