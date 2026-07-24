package models

import "testing"

func TestProfileOrEmpty_KnownKey(t *testing.T) {
	p, ok := ProfileOrEmpty("claude_code")
	if !ok {
		t.Fatal("expected claude_code to be known")
	}
	if p.Command != "claude" {
		t.Errorf("Command = %q, want %q", p.Command, "claude")
	}
	if p.CredentialProvider != "cli:claude" {
		t.Errorf("CredentialProvider = %q, want %q", p.CredentialProvider, "cli:claude")
	}
}

func TestProfileOrEmpty_UnknownKey(t *testing.T) {
	_, ok := ProfileOrEmpty("not_real")
	if ok {
		t.Fatal("expected not_real to be unknown")
	}
}
