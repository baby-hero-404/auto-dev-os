package tool

import (
	"regexp"
	"strconv"
	"strings"
)

var goCompilerErrorRegex = regexp.MustCompile(`^([^:\s]+):(\d+):(?:\d+:)?\s*(.*)$`)

// ParseGoBuildOutput parses combined stdout+stderr from a `go build` (or
// compatible) invocation into file/line diagnostics — shared by
// verify.CompileCheckHook (post-edit verify pipeline) and RunBuildTool
// (agent-facing tool), which previously each carried their own copy of this
// regex and parse loop. Lines that don't match the "file:line: message"
// shape are silently skipped; callers decide what to do if the result is
// empty (their fallback-message behavior differs slightly by design, so
// that part stays with each caller rather than being unified here).
func ParseGoBuildOutput(stdout, stderr string) []Diagnostic {
	var diags []Diagnostic
	outputLines := strings.Split(stdout+"\n"+stderr, "\n")
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := goCompilerErrorRegex.FindStringSubmatch(line)
		if len(matches) == 4 {
			lineNum, _ := strconv.Atoi(matches[2])
			diags = append(diags, Diagnostic{
				Severity: "error",
				File:     matches[1],
				Line:     lineNum,
				Message:  matches[3],
			})
		}
	}
	return diags
}
