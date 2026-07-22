package outputfilter

import "regexp"

// ansiPattern matches ANSI/VT100 escape sequences (color codes, cursor moves, etc).
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// stripANSI removes terminal escape codes and collapses carriage-return progress-bar
// rewrites, keeping only the final state of each \r-rewritten line (REQ-003).
func stripANSI(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = ansiPattern.ReplaceAllString(line, "")
		if idx := lastIndexByte(line, '\r'); idx >= 0 {
			line = line[idx+1:]
		}
		out = append(out, line)
	}
	return out
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
