package patch

import "testing"

func TestIsUnderAffectedDir_AllowsSiblingFileInSameDirectory(t *testing.T) {
	if !IsUnderAffectedDir("pkg/scheduler/helper.go", []string{"pkg/scheduler/scheduler.go"}) {
		t.Fatal("expected sibling file in same directory to be allowed")
	}
}

func TestIsUnderAffectedDir_AllowsWildcardParentDirectory(t *testing.T) {
	if !IsUnderAffectedDir("pkg/scheduler/helper.go", []string{"pkg/scheduler/*.go"}) {
		t.Fatal("expected wildcard pattern to allow files in the same directory")
	}
}

func TestIsUnderAffectedDir_AllowsRootLevelSiblingFile(t *testing.T) {
	if !IsUnderAffectedDir("notes.go", []string{"README.md"}) {
		t.Fatal("expected root-level sibling file to be allowed")
	}
}

func TestIsUnderAffectedDir_RejectsUnrelatedDirectory(t *testing.T) {
	if IsUnderAffectedDir("pkg/api/helper.go", []string{"pkg/scheduler/*.go"}) {
		t.Fatal("expected unrelated directory to be rejected")
	}
}
