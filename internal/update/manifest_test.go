package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func testManifest(version string) *Manifest {
	return &Manifest{
		Product:    product,
		Version:    version,
		ReleasedAt: "2026-08-15T00:00:00Z",
		Platforms: map[string]map[string]PlatformArt{
			"macos": {
				"aarch64": {Sidecars: "rmb-desktop_" + version + "_darwin_arm64.tar.gz", SHA256: "aa"},
			},
			"windows": {
				"amd64": {Sidecars: "rmb-desktop_" + version + "_windows_amd64.zip", SHA256: "bb"},
			},
		},
	}
}

// canonical must be stable under JSON re-serialization and map iteration.
func TestCanonicalStable(t *testing.T) {
	m := testManifest("0.2.0")
	first := string(m.Canonical())

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var re Manifest
	if err := json.Unmarshal(raw, &re); err != nil {
		t.Fatal(err)
	}
	if second := string(re.Canonical()); second != first {
		t.Errorf("canonical changed across marshal roundtrip:\n%s\n%s", first, second)
	}
}

func TestSignatureRoundtripAndTamper(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	m := testManifest("0.2.0")
	m.Sign(priv)

	if !m.VerifyWith(pub) {
		t.Fatal("signed manifest must verify")
	}

	// Tamper with the version → signature must fail.
	tampered := testManifest("0.9.9")
	tampered.Signature = m.Signature
	if tampered.VerifyWith(pub) {
		t.Fatal("tampered manifest must not verify")
	}

	// Tamper with a sha256 → fail.
	m2 := testManifest("0.2.0")
	m2.Sign(priv)
	macos := map[string]PlatformArt{}
	for k, v := range m2.Platforms["macos"] {
		macos[k] = v
	}
	art := macos["aarch64"]
	art.SHA256 = "evil"
	macos["aarch64"] = art
	m2.Platforms["macos"] = macos
	if m2.VerifyWith(pub) {
		t.Fatal("tampered sha256 must not verify")
	}

	// Wrong key → fail.
	otherPub, _, _ := ed25519.GenerateKey(nil)
	if m.VerifyWith(otherPub) {
		t.Fatal("wrong key must not verify")
	}

	// Unsigned → never verifies.
	var m3 Manifest
	if m3.VerifyWith(pub) {
		t.Fatal("unsigned must not verify")
	}
}

// writeSigned serves m at /latest.json, signed with priv.

func TestCheckAgainstTestServer(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	savedB64 := publicKeyB64
	publicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	defer func() {
		publicKeyB64 = savedB64
		pubOnce = sync.Once{}
	}()

	newer := testManifest("0.9.0")
	newer.Sign(priv)
	same := testManifest("0.1.0")
	same.Sign(priv)
	unsigned := testManifest("0.9.0")

	var payload []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest.json" {
			w.Write(payload)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Newer + signed → release found.
	payload = mustJSON(t, newer)
	rel, err := Check(t.Context(), []string{srv.URL + "/latest.json"}, "0.1.0")
	if err != nil || rel == nil {
		t.Fatalf("want release, got rel=%v err=%v", rel, err)
	}
	if rel.Manifest.Version != "0.9.0" {
		t.Fatalf("version = %s", rel.Manifest.Version)
	}
	if want := srv.URL + "/rmb-desktop_0.9.0_darwin_arm64.tar.gz"; rel.BundleURL() != want {
		t.Fatalf("bundle url = %s, want %s", rel.BundleURL(), want)
	}

	// Same version + signed → up to date.
	payload = mustJSON(t, same)
	rel, err = Check(t.Context(), []string{srv.URL + "/latest.json"}, "0.1.0")
	if err != nil || rel != nil {
		t.Fatalf("want nil release, got rel=%v err=%v", rel, err)
	}

	// Unsigned → rejected, falls through to error.
	payload = mustJSON(t, unsigned)
	rel, err = Check(t.Context(), []string{srv.URL + "/latest.json"}, "0.1.0")
	if err == nil {
		t.Fatalf("unsigned manifest must be rejected, got rel=%v", rel)
	}

	// Feed order: bad first, good second still succeeds.
	payload = mustJSON(t, newer)
	rel, err = Check(t.Context(), []string{srv.URL + "/missing.json", srv.URL + "/latest.json"}, "0.1.0")
	if err != nil || rel == nil {
		t.Fatalf("fallback feed failed: rel=%v err=%v", rel, err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
