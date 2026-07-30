package models

import (
	"strings"
	"testing"
)

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

// TestProfileOrEmpty_Antigravity_PBeforePromptArg guards the fix for a
// confirmed bug: agy's flag parser is Go stdlib `flag`, where -p is a value
// flag — whatever token immediately follows -p becomes its value. A boolean
// flag placed after -p (e.g. --dangerously-skip-permissions) gets consumed
// as the prompt text instead of the actual instruction, confirmed from a
// real run's captured output (see docs/guides/antigravity-cli-headless.md
// and cli_profiles.go's antigravity comment). Boolean flags must precede -p.
func TestProfileOrEmpty_Antigravity_PBeforePromptArg(t *testing.T) {
	p, ok := ProfileOrEmpty("antigravity")
	if !ok {
		t.Fatal("expected antigravity to be known")
	}
	if p.Command != "agy" {
		t.Errorf("Command = %q, want %q (the real headless CLI binary)", p.Command, "agy")
	}
	pIdx := -1
	for i, a := range p.Args {
		if a == "-p" {
			pIdx = i
		}
	}
	if pIdx == -1 {
		t.Fatalf("Args %v: expected a -p flag", p.Args)
	}
	if pIdx+1 >= len(p.Args) || strings.HasPrefix(p.Args[pIdx+1], "-") {
		t.Errorf("Args %v: the token immediately after -p must be the prompt text, not another flag", p.Args)
	}
}

func TestProfileOrEmpty_UnknownKey(t *testing.T) {
	_, ok := ProfileOrEmpty("not_real")
	if ok {
		t.Fatal("expected not_real to be unknown")
	}
}
