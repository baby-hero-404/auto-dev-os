package models

import "testing"

func TestAutoEnableExecutionProviderRow_EmptyListScaffolds(t *testing.T) {
	list, changed := AutoEnableExecutionProviderRow(nil, "anthropic")
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(list) != 7 {
		t.Fatalf("expected 7-row scaffold, got %d", len(list))
	}
	for _, p := range list {
		want := p.Type == "api" && p.Ref == "anthropic"
		if p.Enabled != want {
			t.Errorf("row %s:%s enabled=%v, want %v", p.Type, p.Ref, p.Enabled, want)
		}
	}
}

func TestAutoEnableExecutionProviderRow_PreservesPriority(t *testing.T) {
	existing := []ExecutionProviderConfig{
		{Type: "api", Ref: "openai", Priority: 0, Enabled: true},
		{Type: "cli", Ref: "claude_code", Priority: 1, Enabled: false},
	}
	list, changed := AutoEnableExecutionProviderRow(existing, "cli:claude")
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(list) != 2 {
		t.Fatalf("expected list length unchanged at 2, got %d", len(list))
	}
	if list[1].Priority != 1 || !list[1].Enabled {
		t.Errorf("expected claude_code row to stay priority 1 and become enabled, got %+v", list[1])
	}
	if list[0].Priority != 0 || !list[0].Enabled {
		t.Errorf("expected openai row untouched, got %+v", list[0])
	}
}

func TestAutoEnableExecutionProviderRow_AlreadyEnabledNoop(t *testing.T) {
	existing := []ExecutionProviderConfig{
		{Type: "api", Ref: "gemini", Priority: 0, Enabled: true},
	}
	list, changed := AutoEnableExecutionProviderRow(existing, "gemini")
	if changed {
		t.Fatal("expected changed=false for already-enabled row")
	}
	if !list[0].Enabled {
		t.Errorf("row should remain enabled")
	}
}

func TestAutoEnableExecutionProviderRow_UnknownProviderNoop(t *testing.T) {
	existing := []ExecutionProviderConfig{{Type: "api", Ref: "anthropic", Priority: 0, Enabled: false}}
	list, changed := AutoEnableExecutionProviderRow(existing, "cli:some-unknown-tool")
	if changed {
		t.Fatal("expected changed=false for unmapped provider")
	}
	if len(list) != 1 {
		t.Fatalf("expected list untouched, got %+v", list)
	}
}

func TestAutoEnableExecutionProviderRow_CustomCLINeverAutoEnabled(t *testing.T) {
	// A "cli:custom-something" credential provider doesn't map to any known
	// row — custom CLI requires hand-configured command/args and must stay
	// manual, per executionProviderRowForCredential's contract.
	list, changed := AutoEnableExecutionProviderRow(nil, "cli:custom-something")
	if changed || list != nil {
		t.Fatalf("expected no-op for unmapped custom-ish provider, got list=%+v changed=%v", list, changed)
	}
}

func TestAutoEnableExecutionProviderRow_MissingRowAppended(t *testing.T) {
	existing := []ExecutionProviderConfig{{Type: "api", Ref: "openai", Priority: 0, Enabled: true}}
	list, changed := AutoEnableExecutionProviderRow(existing, "anthropic")
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(list) != 2 || list[1].Type != "api" || list[1].Ref != "anthropic" || !list[1].Enabled {
		t.Fatalf("expected anthropic row appended and enabled, got %+v", list)
	}
}

func TestDefaultExecutionProviderRows_MatchesFrontendOrder(t *testing.T) {
	rows := DefaultExecutionProviderRows()
	want := []struct{ Type, Ref string }{
		{"cli", "claude_code"},
		{"cli", "antigravity"},
		{"cli", "openai_codex"},
		{"cli", "custom"},
		{"api", "anthropic"},
		{"api", "openai"},
		{"api", "gemini"},
	}
	if len(rows) != len(want) {
		t.Fatalf("expected %d rows, got %d", len(want), len(rows))
	}
	for i, w := range want {
		if rows[i].Type != w.Type || rows[i].Ref != w.Ref || rows[i].Priority != i || rows[i].Enabled {
			t.Errorf("row %d: got %+v, want type=%s ref=%s priority=%d enabled=false", i, rows[i], w.Type, w.Ref, i)
		}
	}
}
