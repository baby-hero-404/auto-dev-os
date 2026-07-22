package outputfilter

import (
	"strconv"
	"strings"
	"testing"
)

func TestDedupLines_ThresholdThree(t *testing.T) {
	lines := []string{"a", "dup", "dup", "dup", "b"}
	out := dedupLines(lines)
	want := []string{"a", "dup", "[repeated 3 times]", "b"}
	if !equal(out, want) {
		t.Errorf("got %v, want %v", out, want)
	}
}

func TestDedupLines_TwoRepeatsNotCollapsed(t *testing.T) {
	lines := []string{"a", "dup", "dup", "b"}
	out := dedupLines(lines)
	if !equal(out, lines) {
		t.Errorf("expected 2 repeats to be left untouched, got %v", out)
	}
}

func TestStripANSI_RemovesColorCodes(t *testing.T) {
	line := "\x1b[31merror\x1b[0m: build failed"
	out := stripANSI([]string{line})
	if out[0] != "error: build failed" {
		t.Errorf("got %q", out[0])
	}
}

func TestStripANSI_CollapsesCarriageReturnProgress(t *testing.T) {
	line := "10%\r50%\r100% done"
	out := stripANSI([]string{line})
	if out[0] != "100% done" {
		t.Errorf("expected only final \\r segment kept, got %q", out[0])
	}
}

func TestErrorPriorityTruncate_KeepsErrorContextAndHeadTail(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "noise line "+strconv.Itoa(i))
	}
	lines[50] = "FATAL: something broke"
	out := errorPriorityTruncate(lines, 200) // tiny budget forces truncation
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "FATAL: something broke") {
		t.Errorf("expected error line to survive truncation, got:\n%s", joined)
	}
	if !strings.Contains(joined, "lines omitted") {
		t.Errorf("expected an omitted-lines marker, got:\n%s", joined)
	}
	if !strings.Contains(joined, "noise line 0") {
		t.Errorf("expected head lines to survive, got:\n%s", joined)
	}
	if !strings.Contains(joined, "noise line 99") {
		t.Errorf("expected tail lines to survive, got:\n%s", joined)
	}
}

func TestErrorPriorityTruncate_NoopWhenUnderBudget(t *testing.T) {
	lines := []string{"a", "b", "c"}
	out := errorPriorityTruncate(lines, 1000)
	if !equal(out, lines) {
		t.Errorf("expected no change under budget, got %v", out)
	}
}

func TestErrorPriorityTruncate_NoErrorLines_KeepsHeadAndTail(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	out := errorPriorityTruncate(lines, 200)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "line 0") || !strings.Contains(joined, "line 99") {
		t.Errorf("expected head+tail kept with no error lines, got:\n%s", joined)
	}
	if !strings.Contains(joined, "lines omitted") {
		t.Errorf("expected omitted marker for the cut middle, got:\n%s", joined)
	}
}

func TestRun_LineSubsequenceInvariant(t *testing.T) {
	// Every non-marker line in the filtered output must appear, byte-for-byte, in the
	// original input (REQ-007): filters only delete/merge/mark, never rewrite content.
	input := "\x1b[31mERROR: boom\x1b[0m\n" + strings.Repeat("same line\n", 5) + "trailing context\n"
	filtered, _ := Run("run_build", input, 40)
	inputSet := make(map[string]bool)
	for _, l := range strings.Split(input, "\n") {
		inputSet[ansiPattern.ReplaceAllString(strings.TrimRight(l, "\r"), "")] = true
	}
	for _, l := range strings.Split(filtered, "\n") {
		if strings.HasPrefix(l, "[repeated ") || strings.HasPrefix(l, "[... ") || l == "" {
			continue
		}
		if !inputSet[l] {
			t.Errorf("filtered line %q not found verbatim (post-ANSI-strip) in input", l)
		}
	}
}

func TestRun_DiffProfileSkipsDedup(t *testing.T) {
	input := strings.Repeat("+same\n", 5)
	filtered, _ := Run("git_diff", input, 10000)
	if strings.Contains(filtered, "repeated") {
		t.Errorf("expected diff profile to skip dedup, got: %s", filtered)
	}
}

func TestRun_ReadProfileNoFiltering(t *testing.T) {
	input := "\x1b[31mcolored\x1b[0m\n" + strings.Repeat("dup\n", 5)
	filtered, _ := Run("read_file", input, 10000)
	if filtered != strings.TrimRight(input, "\n") && filtered != input {
		// Run joins with "\n" without a trailing newline; compare against the
		// no-trailing-newline form.
		if filtered != strings.Join(strings.Split(input, "\n"), "\n") {
			t.Errorf("expected read profile to leave content untouched, got: %q", filtered)
		}
	}
}

func TestProfileFor_UnknownToolGetsDefault(t *testing.T) {
	p := ProfileFor("some_unregistered_tool")
	if p.Name != "default" {
		t.Errorf("expected default profile, got %s", p.Name)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
