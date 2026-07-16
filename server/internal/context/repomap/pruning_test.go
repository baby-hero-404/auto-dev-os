package repomap

import (
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
	"strings"
	"testing"
)

func TestTokenPruning(t *testing.T) {
	tags := []source.Tag{
		{Name: "CoreFunc", Kind: "def", Filepath: "core.go", Line: 10},
		{Name: "HelperFunc", Kind: "def", Filepath: "utils.go", Line: 5},
		{Name: "UnusedFunc", Kind: "def", Filepath: "utils.go", Line: 20},
	}

	pageRank := map[string]float64{
		"core.go":  0.9,
		"utils.go": 0.5,
	}

	// Mock token counter: 1 word = 1 token for predictable testing
	mockCountFn := func(text string) int {
		return len(strings.Fields(text))
	}

	// Tight token budget: 5 tokens (e.g. "core.go:" "def" "CoreFunc" "(1" "lines)")
	// Expected to drop utils.go entirely because core.go has higher PageRank
	res := PruneTags(tags, pageRank, 5, FormatSkeleton, mockCountFn)

	if !strings.Contains(res, "core.go") {
		t.Error("Pruned result should contain high rank file core.go")
	}

	if strings.Contains(res, "utils.go") {
		t.Error("Pruned result should drop low rank file utils.go due to token limits")
	}
}

func TestSkeletonFormatter(t *testing.T) {
	tags := []source.Tag{
		{Name: "Auth", Kind: "def", Filepath: "auth.go", Line: 10},
	}

	res := FormatSkeleton(tags)
	expected := "auth.go:\n  def Auth (1 lines)"

	if res != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, res)
	}
}

// TestSkeletonFormatter_OmitsRefs guards against reintroducing "ref" lines into the rendered
// skeleton. A function's refs (every call it makes internally) reveal its call sequence, which
// violates the "no code bodies" acceptance criterion (docs/features/engineering/
// 01-context-management.md AC-4) and, for functions with many calls, can bloat the token budget
// enough to crowd out other files' "def" signatures entirely (see PruneTags — tags sharing a
// file's PageRank score sort together, so a function's dozens of refs consume budget slots ahead
// of a different, possibly more relevant file). Refs are still extracted and used elsewhere
// (graph.BuildGraph's edge weights) — they just must not appear in this rendered output.
func TestSkeletonFormatter_OmitsRefs(t *testing.T) {
	tags := []source.Tag{
		{Name: "GetCommits", Kind: "def", Filepath: "client.go", Line: 10},
		{Name: "Errorf", Kind: "ref", Filepath: "client.go", Line: 12},
		{Name: "NewRequestWithContext", Kind: "ref", Filepath: "client.go", Line: 14},
	}

	res := FormatSkeleton(tags)

	if strings.Contains(res, "ref ") {
		t.Errorf("expected no \"ref\" lines in the rendered skeleton, got:\n%s", res)
	}
	if !strings.Contains(res, "def GetCommits") {
		t.Errorf("expected the def line to still be present, got:\n%s", res)
	}
}
