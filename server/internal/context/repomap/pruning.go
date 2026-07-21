package repomap

import (
	"sort"

	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
	"github.com/pkoukk/tiktoken-go"
)

type ScoredTag struct {
	Tag   source.Tag
	Score float64
}

var globalTkm *tiktoken.Tiktoken

func init() {
	globalTkm, _ = tiktoken.GetEncoding("cl100k_base")
}

// CountTokens exactly counts tokens using OpenAI's cl100k_base encoding.
func CountTokens(text string) int {
	if globalTkm == nil {
		// Fallback estimation (1 token ~= 4 chars)
		return len(text) / 4
	}
	token := globalTkm.Encode(text, nil, nil)
	return len(token)
}

// PruneTags uses binary search to find the Top N tags that perfectly fit within maxTokens.
func PruneTags(tags []source.Tag, pageRank map[string]float64, maxTokens int, formatFn func([]source.Tag) string, countFn func(string) int) string {
	if maxTokens <= 0 {
		return ""
	}

	// 1. Sort all tags based on the PageRank score of their owning file
	var scored []ScoredTag
	for _, t := range tags {
		score := pageRank[t.Filepath]

		// Priority boost: Definitions are slightly more important than References
		if t.Kind == "def" {
			score *= 1.1
		}

		scored = append(scored, ScoredTag{Tag: t, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 2. Binary search to find optimal tag count
	low := 0
	high := len(scored)
	var bestResult string

	for low <= high {
		mid := (low + high) / 2

		// Slice top 'mid' tags
		subset := make([]source.Tag, mid)
		for i := 0; i < mid; i++ {
			subset[i] = scored[i].Tag
		}

		text := formatFn(subset)
		tokens := countFn(text)

		if tokens <= maxTokens {
			bestResult = text
			low = mid + 1 // try to fit more tags safely
		} else {
			high = mid - 1 // too big, shrink slice
		}
	}

	return bestResult
}
