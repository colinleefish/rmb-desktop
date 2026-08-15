// Command manifest-sign creates and signs update manifests for rmb-desktop
// releases (Phase 2 of plan/tauri-to-go-shell.md). Not shipped to users.
//
// Usage:
//
//	manifest-sign genkey <keyfile>       generate ed25519 keypair (private seed file + pubkey printout)
//	manifest-sign sign <manifest.json>   inject signature using $RMB_MANIFEST_KEY or ~/.rmb/release-keys/manifest.key
//	manifest-sign verify <manifest.json> verify against the public key embedded in internal/update
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		usage()
	}
	switch os.Args[1] {
	case "genkey":
		if err := genkey(os.Args[2]); err != nil {
			fail(err)
		}
	case "sign":
		if err := sign(os.Args[2]); err != nil {
			fail(err)
		}
	case "verify":
		if err := verify(os.Args[2]); err != nil {
			fail(err)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: manifest-sign genkey <keyfile> | sign <manifest.json> | verify <manifest.json>")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "manifest-sign:", err)
	os.Exit(1)
}

func defaultKeyPath() string {
	if p := os.Getenv("RMB_MANIFEST_KEY"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rmb", "release-keys", "manifest.key")
}

func genkey(path string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	seed := priv.Seed()
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("private seed: %s (0600)\n", path)
	fmt.Printf("\nAdd to internal/update/pubkey.go:\n\nvar publicKeyB64 = %q\n\n", base64.StdEncoding.EncodeToString(pub))
	return nil
}

func loadPrivateKey() (ed25519.PrivateKey, error) {
	path := defaultKeyPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s (set RMB_MANIFEST_KEY or run genkey): %w", path, err)
	}
	seed, err := base64.StdEncoding.DecodeString(trimSpaceString(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s: want %d-byte seed, got %d", path, ed25519.SeedSize, len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func sign(path string) error {
	m, err := readManifest(path)
	if err != nil {
		return err
	}
	priv, err := loadPrivateKey()
	if err != nil {
		return err
	}
	m.Sign(priv)
	return writeManifest(path, m)
}

func verify(path string) error {
	m, err := readManifest(path)
	if err != nil {
		return err
	}
	if err := m.Verify(); err != nil {
		return err
	}
	fmt.Printf("manifest-sign: %s v%s signature OK\n", m.Product, m.Version)
	return nil
}

func trimSpaceString(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
