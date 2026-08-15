package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"sync"
)

// publicKeyB64 is the release manifest signing key (ed25519, base64).
// Private seed lives in the release keychain; see cmd/manifest-sign genkey.
var publicKeyB64 = "PnAnRehb4AkfhNiY5jUukpxuP+hyAOHBjrJcCUdnwy0="

var (
	pubOnce sync.Once
	pub     ed25519.PublicKey
	pubErr  error
)

func publicKey() (ed25519.PublicKey, error) {
	pubOnce.Do(func() {
		raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			pubErr = fmt.Errorf("update: invalid embedded public key (len %d)", len(raw))
			return
		}
		pub = ed25519.PublicKey(raw)
	})
	return pub, pubErr
}
