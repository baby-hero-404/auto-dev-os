package workflow

import (
	"strings"
	"testing"
)

func TestParseTasksMD_ValidMarkdown(t *testing.T) {
	md := `## 1. Backend API
- [ ] 1.1 Create API endpoint for users
- [ ] 1.2 Add input validation

## 2. Frontend Components
- [ ] 2.1 Build user form component
- [ ] 2.2 Add responsive styles
`
	result := ParseTasksMD(md)

	if len(result["backend"]) != 1 {
		t.Errorf("expected 1 backend task group, got %d: %v", len(result["backend"]), result["backend"])
	}
	if len(result["frontend"]) != 1 {
		t.Errorf("expected 1 frontend task group, got %d: %v", len(result["frontend"]), result["frontend"])
	}
	if !strings.Contains(result["backend"][0], "1.1 Create API endpoint for users") {
		t.Errorf("unexpected backend task: %q", result["backend"][0])
	}
	if !strings.Contains(result["frontend"][0], "2.1 Build user form component") {
		t.Errorf("unexpected frontend task: %q", result["frontend"][0])
	}
}

func TestParseTasksMD_Empty(t *testing.T) {
	result := ParseTasksMD("")
	if len(result) != 0 {
		t.Errorf("expected empty map for empty input, got %v", result)
	}

	result = ParseTasksMD("   \n  \n  ")
	if len(result) != 0 {
		t.Errorf("expected empty map for whitespace input, got %v", result)
	}
}

func TestParseTasksMD_NoCheckboxes(t *testing.T) {
	md := `## Backend
Some prose description here.
More text without checkboxes.
`
	result := ParseTasksMD(md)
	if len(result["backend"]) != 0 {
		t.Errorf("expected 0 backend tasks without checkboxes, got %d", len(result["backend"]))
	}
}

func TestParseTasksMD_SingleGroup(t *testing.T) {
	md := `## Database Migration
- [ ] Create migration for users table
- [x] Add index on email column
`
	result := ParseTasksMD(md)
	if len(result["backend"]) != 1 {
		t.Errorf("expected 1 backend task group, got %d: %v", len(result["backend"]), result)
	}
	if len(result["frontend"]) != 0 {
		t.Errorf("expected 0 frontend tasks, got %d", len(result["frontend"]))
	}
}

func TestParseTasksMD_VietnameseHeaders(t *testing.T) {
	md := `## 1. Xử lý API Backend
- [ ] Tạo endpoint cho users
- [ ] Thêm validation

## 2. Giao diện người dùng
- [ ] Tạo form đăng ký
`
	result := ParseTasksMD(md)
	if len(result["backend"]) != 1 {
		t.Errorf("expected 1 backend task group, got %d", len(result["backend"]))
	}
	if len(result["frontend"]) != 1 {
		t.Errorf("expected 1 frontend task group, got %d", len(result["frontend"]))
	}
}

func TestParseCheckboxes_FormatVariants(t *testing.T) {
	md := "- [ ] item one\n* [x] item two\n-   [X]  item three\n"
	done, total := ParseCheckboxes(md)
	if total != 3 {
		t.Fatalf("expected 3 checkboxes, got %d", total)
	}
	if done != 2 {
		t.Fatalf("expected 2 done, got %d", done)
	}
}

func TestParseCheckboxes_IgnoresFencedCodeBlocks(t *testing.T) {
	md := "- [x] real task\n\n```markdown\n- [ ] example in a fence\n- [x] another example\n```\n- [ ] another real task\n"
	done, total := ParseCheckboxes(md)
	if total != 2 {
		t.Fatalf("expected 2 checkboxes (fenced ones excluded), got %d", total)
	}
	if done != 1 {
		t.Fatalf("expected 1 done, got %d", done)
	}
}

func TestParseCheckboxes_Empty(t *testing.T) {
	done, total := ParseCheckboxes("")
	if done != 0 || total != 0 {
		t.Fatalf("expected 0/0 for empty input, got %d/%d", done, total)
	}
}

func TestParseTasksMD_DefaultClassification(t *testing.T) {
	md := `## 1. Testing & Quality
- [ ] Write unit tests
- [ ] Add integration tests
`
	result := ParseTasksMD(md)
	if len(result["backend"]) != 1 {
		t.Errorf("expected 1 backend task group, got %d", len(result["backend"]))
	}
}
