package codecontext

import (
	"fmt"
	"os"
)

// ContextBudget defines hard limits to prevent context token explosion.
type ContextBudget struct {
	MaxFiles        int
	MaxBytesPerFile int
	MaxTotalBytes   int
}

// DefaultBudget returns sensible defaults for the 4-tier retriever.
func DefaultBudget() ContextBudget {
	return ContextBudget{
		MaxFiles:        8,
		MaxBytesPerFile: 20_000,
		MaxTotalBytes:   80_000,
	}
}

func readFileBudget(path string, maxBytes int) (string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("is a directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	if len(data) <= maxBytes {
		return string(data), false, nil
	}

	// Truncate and add marker.
	truncated := string(data[:maxBytes]) + "\n// ... truncated (file exceeds budget) ...\n"
	return truncated, true, nil
}
