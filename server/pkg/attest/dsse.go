package attest

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
)

// PayloadType is the DSSE payload type for an in-toto statement.
const PayloadType = "application/vnd.in-toto+json"

// Signature is one entry in an Envelope's signatures list, keyed by the
// signing key's key_id so a multi-key deployment (rotation) can look up
// the right public key to verify against (REQ-004/REQ-006).
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"` // base64
}

// Envelope is a DSSE envelope (https://github.com/secure-systems-lab/dsse).
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"` // base64 of the raw statement JSON
	Signatures  []Signature `json:"signatures"`
}

// pae implements the DSSE Pre-Authentication Encoding (PAE):
// PAE(type, body) = "DSSEv1" + SP + LEN(type) + SP + type + SP + LEN(body) + SP + body
func pae(payloadType string, body []byte) []byte {
	out := []byte("DSSEv1 ")
	out = append(out, strconv.Itoa(len(payloadType))...)
	out = append(out, ' ')
	out = append(out, payloadType...)
	out = append(out, ' ')
	out = append(out, strconv.Itoa(len(body))...)
	out = append(out, ' ')
	out = append(out, body...)
	return out
}

// Sign builds a DSSE envelope around statement, signed with priv under keyID.
func Sign(statement *Statement, keyID string, priv ed25519.PrivateKey) (*Envelope, error) {
	body, err := json.Marshal(statement)
	if err != nil {
		return nil, fmt.Errorf("marshal statement: %w", err)
	}
	msg := pae(PayloadType, body)
	sig := ed25519.Sign(priv, msg)
	return &Envelope{
		PayloadType: PayloadType,
		Payload:     base64.StdEncoding.EncodeToString(body),
		Signatures: []Signature{{
			KeyID: keyID,
			Sig:   base64.StdEncoding.EncodeToString(sig),
		}},
	}, nil
}

// Verify checks env's signature from keyID against pub. It returns an error
// if the envelope has no signature for keyID or the signature does not
// verify (e.g. the payload was tampered with).
func Verify(env *Envelope, keyID string, pub ed25519.PublicKey) error {
	body, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	var sigB64 string
	found := false
	for _, s := range env.Signatures {
		if s.KeyID == keyID {
			sigB64 = s.Sig
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no signature found for key_id %q", keyID)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	msg := pae(env.PayloadType, body)
	if !ed25519.Verify(pub, msg, sig) {
		return fmt.Errorf("signature verification failed for key_id %q", keyID)
	}
	return nil
}

// DecodeStatement decodes env's payload back into a Statement.
func DecodeStatement(env *Envelope) (*Statement, error) {
	body, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var st Statement
	if err := json.Unmarshal(body, &st); err != nil {
		return nil, fmt.Errorf("unmarshal statement: %w", err)
	}
	return &st, nil
}
