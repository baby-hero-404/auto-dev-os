package repomap

import (
	"strings"
	"testing"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
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
	
	// Tight token budget: 4 tokens (e.g. "core.go:" "def" "CoreFunc")
	// Expected to drop utils.go entirely because core.go has higher PageRank
	res := PruneTags(tags, pageRank, 4, FormatSkeleton, mockCountFn)
	
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
	expected := "auth.go:\n  def Auth"
	
	if res != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, res)
	}
}
