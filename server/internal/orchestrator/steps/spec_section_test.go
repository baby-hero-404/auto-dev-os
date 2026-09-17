package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpecSet(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBuildSpecSection_ExtractsGoalDecisionsAndVerify(t *testing.T) {
	dir := writeSpecSet(t, map[string]string{
		"proposal.md": "# Proposal\n\n## Goal\n\nMake the widget resizable.\n\n## Decisions\n\n- Use CSS resize rather than a JS handler\n- Keep the minimum width at 320px\n",
		"specs.md":    "# Expected Behavior\n\n## Verify\n\n- Drag the widget edge and confirm it resizes\n- Confirm it will not shrink below 320px\n",
	})

	section := BuildSpecSection("resizable-widget", dir)
	if section.Missing {
		t.Fatal("expected a parsed spec, got Missing")
	}
	if !strings.Contains(section.Goal, "resizable") {
		t.Errorf("expected the goal to be extracted, got %q", section.Goal)
	}
	if len(section.KeyDecisions) != 2 {
		t.Errorf("expected 2 key decisions, got %v", section.KeyDecisions)
	}
	if len(section.VerifySteps) != 2 {
		t.Errorf("expected 2 verify steps, got %v", section.VerifySteps)
	}

	body := section.Render()
	for _, want := range []string{"OpenSpec", "docs/openspecs/resizable-widget/", "resizable", "CSS resize", "320px"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected rendered body to contain %q, got:\n%s", want, body)
		}
	}
}

// A Small task writes a single spec.md instead of the proposal/specs split.
func TestBuildSpecSection_SingleFileSpec(t *testing.T) {
	dir := writeSpecSet(t, map[string]string{
		"spec.md": "# Spec\n\n## Goal\n\nFix the off-by-one.\n\n## Verify\n\n- Run the boundary test\n",
	})
	section := BuildSpecSection("off-by-one", dir)
	if section.Missing {
		t.Fatal("expected a single-file spec to parse")
	}
	if section.Goal != "Fix the off-by-one." {
		t.Errorf("unexpected goal %q", section.Goal)
	}
	if len(section.VerifySteps) != 1 {
		t.Errorf("expected 1 verify step, got %v", section.VerifySteps)
	}
}

// A missing spec must not block the MR, but it must be stated loudly — the MR
// is the only place a reviewer would otherwise notice nothing was planned.
func TestBuildSpecSection_MissingDirRendersWarning(t *testing.T) {
	section := BuildSpecSection("nope", filepath.Join(t.TempDir(), "does-not-exist"))
	if !section.Missing {
		t.Fatal("expected Missing for an absent spec directory")
	}
	body := section.Render()
	if !strings.Contains(body, "No OpenSpec was produced") {
		t.Errorf("expected an explicit no-spec notice, got:\n%s", body)
	}
}

// A directory that exists but yields nothing parseable is the same problem for
// a reviewer as no spec at all, and must not render a reassuring empty block.
func TestBuildSpecSection_UnparseableTreatedAsMissing(t *testing.T) {
	dir := writeSpecSet(t, map[string]string{"proposal.md": "just some prose with no headings at all\n"})
	section := BuildSpecSection("blank", dir)
	if !section.Missing {
		t.Error("expected an unparseable spec set to be reported as missing")
	}
}

func TestSpecSection_RenderRedactsSecrets(t *testing.T) {
	dir := writeSpecSet(t, map[string]string{
		"proposal.md": "## Goal\n\nRotate the token ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa now.\n",
	})
	body := BuildSpecSection("rotate", dir).Render()
	if strings.Contains(body, "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Error("a credential in spec text must not reach the MR body")
	}
	if !strings.Contains(body, "[REDACTED]") {
		t.Errorf("expected the secret to be redacted, got:\n%s", body)
	}
}
