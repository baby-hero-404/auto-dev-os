// Package outputfilter implements a deterministic, non-LLM filtering pipeline for tool
// output before it is handed to the loop's hard-cut truncation. Every filter operates on
// []string lines and never rewrites the bytes of a kept line (REQ-007) — it only removes,
// merges, or annotates with marker lines.
package outputfilter

import "strings"

// Filter transforms a slice of lines into a (possibly shorter/annotated) slice of lines.
type Filter interface {
	Apply(lines []string) []string
}

// BudgetFilter is like Filter but is also given the remaining char budget, since only
// error-priority truncation needs to know how much space is left.
type BudgetFilter interface {
	ApplyBudget(lines []string, budget int) []string
}

// Profile names an ordered filter pipeline for a class of tools.
type Profile struct {
	Name          string
	StripANSI     bool
	Dedup         bool
	PathCompress  bool
	ErrorPriority bool
}

var (
	ProfileBuild   = Profile{Name: "build", StripANSI: true, Dedup: true, PathCompress: true, ErrorPriority: true}
	ProfileTest    = Profile{Name: "test", StripANSI: true, Dedup: true, PathCompress: true, ErrorPriority: true}
	ProfileDiff    = Profile{Name: "diff", StripANSI: true, Dedup: false, PathCompress: false, ErrorPriority: false}
	ProfileRead    = Profile{Name: "read", StripANSI: false, Dedup: false, PathCompress: false, ErrorPriority: false}
	ProfileDefault = Profile{Name: "default", StripANSI: true, Dedup: true, PathCompress: false, ErrorPriority: false}
)

// toolProfiles maps tool names to their declared profile (design.md's per-tool metadata,
// expressed as a registry keyed by tool name instead of touching every tool definition file).
var toolProfiles = map[string]Profile{
	"run_build":  ProfileBuild,
	"run_lint":   ProfileBuild,
	"run_tests":  ProfileTest,
	"git_diff":   ProfileDiff,
	"git_status": ProfileDiff,
	"read_file":  ProfileRead,
}

// ProfileFor returns the declared profile for a tool name, or ProfileDefault if none is
// registered (REQ-004).
func ProfileFor(toolName string) Profile {
	if p, ok := toolProfiles[toolName]; ok {
		return p
	}
	return ProfileDefault
}

// Stats reports the effect of a filter run for the metrics log (REQ-006).
type Stats struct {
	ToolName string
	InBytes  int
	OutBytes int
	SavedPct float64
}

// Run applies toolName's profile pipeline to output, bounded by budget chars, and returns
// the filtered text plus stats. The pipeline order is strip -> dedup -> pathcompress ->
// errorpriority(budget), matching design.md. The caller's own hard-cut remains a safety net
// applied after Run (REQ-005) — Run does not guarantee staying under budget for profiles
// that don't include error-priority truncation (e.g. "diff", "read").
func Run(toolName, output string, budget int) (string, Stats) {
	inBytes := len(output)
	profile := ProfileFor(toolName)

	lines := splitLines(output)

	if profile.StripANSI {
		lines = stripANSI(lines)
	}
	if profile.Dedup {
		lines = dedupLines(lines)
	}
	if profile.PathCompress {
		lines = compressPaths(lines)
	}
	if profile.ErrorPriority {
		lines = errorPriorityTruncate(lines, budget)
	} else if profile.Name == "diff" {
		lines = tailCutIfNeeded(lines, budget)
	}

	filtered := strings.Join(lines, "\n")
	outBytes := len(filtered)
	saved := 0.0
	if inBytes > 0 {
		saved = (1 - float64(outBytes)/float64(inBytes)) * 100
	}
	return filtered, Stats{ToolName: toolName, InBytes: inBytes, OutBytes: outBytes, SavedPct: saved}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
