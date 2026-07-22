package outputfilter

import "fmt"

// dedupRunThreshold is the minimum run length of identical consecutive lines that gets
// collapsed. Runs of 1-2 identical lines are left untouched (REQ-001).
const dedupRunThreshold = 3

// dedupLines collapses runs of >= dedupRunThreshold identical consecutive lines into a
// single copy of the line plus a "[repeated N times]" marker.
func dedupLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		runLen := j - i
		if runLen >= dedupRunThreshold {
			out = append(out, lines[i], fmt.Sprintf("[repeated %d times]", runLen))
		} else {
			for k := i; k < j; k++ {
				out = append(out, lines[k])
			}
		}
		i = j
	}
	return out
}
