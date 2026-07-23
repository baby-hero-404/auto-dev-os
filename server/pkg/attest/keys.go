package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// KeyStatus is a signing key's lifecycle state (REQ-006).
type KeyStatus string

const (
	KeyStatusActive  KeyStatus = "active"
	KeyStatusRetired KeyStatus = "retired"
)

// KeyID derives the stable key_id for a public key: sha256(pub)[:8] hex.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])[:8]
}

// GenerateKeyPair creates a new Ed25519 keypair and its derived key_id.
func GenerateKeyPair() (keyID string, pub ed25519.PublicKey, priv ed25519.PrivateKey, err error) {
	pub, priv, err = ed25519.GenerateKey(nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("generate ed25519 keypair: %w", err)
	}
	return KeyID(pub), pub, priv, nil
}

// JWK is a minimal JSON Web Key representation of an Ed25519 public key,
// sufficient for offline verification (REQ-006's JWKS export).
type JWK struct {
	Kty    string `json:"kty"`
	Crv    string `json:"crv"`
	X      string `json:"x"` // base64url public key bytes
	KeyID  string `json:"kid"`
	Status string `json:"status"`
}

// JWKSet is the JWKS-format list returned by GET /attestations/keys.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}
