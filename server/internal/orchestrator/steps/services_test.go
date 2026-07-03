package steps

import "testing"

func TestUpdateTaskSubtaskMarkdown_ReplacesExactBlock(t *testing.T) {
	before := "## Backend API\n- [ ] Create endpoint\n- [ ] Add validation\n\n## UI\n- [ ] Build form\n"
	block := "## Backend API\n- [ ] Create endpoint\n- [ ] Add validation"

	after, ok := updateTaskSubtaskMarkdown(before, block)
	if !ok {
		t.Fatal("expected block update to succeed")
	}
	if after == before {
		t.Fatal("expected markdown to change")
	}
	expected := "## Backend API\n- [x] Create endpoint\n- [x] Add validation\n\n## UI\n- [ ] Build form\n"
	if after != expected {
		t.Fatalf("unexpected markdown:\n%s", after)
	}
}

func TestUpdateTaskSubtaskMarkdown_DuplicateBlock_SkipsReplace(t *testing.T) {
	before := "## Backend API\n- [ ] Add validation\n\n## Backend API\n- [ ] Add validation\n"
	block := "## Backend API\n- [ ] Add validation"

	after, ok := updateTaskSubtaskMarkdown(before, block)
	if ok {
		t.Fatal("expected duplicate block replacement to be skipped")
	}
	if after != before {
		t.Fatal("expected markdown to remain unchanged")
	}
}
