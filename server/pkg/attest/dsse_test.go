package attest

import (
	"encoding/base64"
	"testing"
	"time"
)

func testStatement() *Statement {
	return BuildStatement("acme/repo", "deadbeef", Predicate{
		CodedBy:    Actor{Engine: "api_native", Provider: "anthropic", Model: "claude-x"},
		PromptHash: "sha256:abc",
		Policy:     PolicySnapshot{Autonomy: "supervised", ReviewHarness: "different_model", FixCyclesUsed: 1},
		TaskID:     "t1",
		JobID:      "j1",
		Timestamp:  time.Unix(0, 0).UTC(),
	})
}

func TestSignAndVerify_RoundTrips(t *testing.T) {
	keyID, pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	env, err := Sign(testStatement(), keyID, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := Verify(env, keyID, pub); err != nil {
		t.Fatalf("expected verify to pass, got: %v", err)
	}
}

func TestVerify_FailsOnTamperedPayload(t *testing.T) {
	keyID, pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	env, err := Sign(testStatement(), keyID, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	raw[0] ^= 0xFF // flip a byte
	env.Payload = base64.StdEncoding.EncodeToString(raw)

	if err := Verify(env, keyID, pub); err == nil {
		t.Fatal("expected verify to fail after tampering with payload, got nil error")
	}
}

func TestVerify_FailsOnWrongKey(t *testing.T) {
	keyID, _, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	env, err := Sign(testStatement(), keyID, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, otherPub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	if err := Verify(env, keyID, otherPub); err == nil {
		t.Fatal("expected verify to fail against a mismatched public key, got nil error")
	}
}

func TestVerify_FailsWhenKeyIDMissing(t *testing.T) {
	keyID, pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	env, err := Sign(testStatement(), keyID, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := Verify(env, "nonexistent", pub); err == nil {
		t.Fatal("expected verify to fail for an unknown key_id, got nil error")
	}
}

func TestDecodeStatement_RoundTrips(t *testing.T) {
	keyID, _, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	original := testStatement()
	env, err := Sign(original, keyID, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	decoded, err := DecodeStatement(env)
	if err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	if decoded.Predicate.TaskID != original.Predicate.TaskID {
		t.Errorf("expected task_id %q, got %q", original.Predicate.TaskID, decoded.Predicate.TaskID)
	}
	if decoded.Subject[0].Digest["gitCommit"] != "deadbeef" {
		t.Errorf("expected commit digest deadbeef, got %q", decoded.Subject[0].Digest["gitCommit"])
	}
}
