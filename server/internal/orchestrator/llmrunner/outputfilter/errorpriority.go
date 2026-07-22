package outputfilter

import (
	"fmt"
	"regexp"
)

// errorLine matches lines likely to carry actionable failure signal (build/test/lint
// output), per design.md v1 pattern list.
var errorLine = regexp.MustCompile(`(?i)\b(error|err!|fail(ed|ure)?|panic|fatal|exception|traceback|undefined|cannot|✗|FAIL)\b`)

// errorContextRadius is how many lines of context are kept on each side of an error line.
const errorContextRadius = 2

// headTailLines is how many lines are always kept from the start and end of the output.
const headTailLines = 20

type lineRange struct{ start, end int } // end exclusive

func mergeRanges(ranges []lineRange) []lineRange {
	if len(ranges) == 0 {
		return nil
	}
	merged := []lineRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.start <= last.end {
			if r.end > last.end {
				last.end = r.end
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// errorPriorityTruncate returns lines unchanged if the joined output already fits budget.
// Otherwise it keeps: every line matching errorLine plus errorContextRadius lines of
// context each side, merged with the first/last headTailLines lines, and replaces omitted
// gaps with a "[... M lines omitted ...]" marker (REQ-002).
func errorPriorityTruncate(lines []string, budget int) []string {
	if budget <= 0 || totalLen(lines) <= budget {
		return lines
	}
	n := len(lines)
	if n == 0 {
		return lines
	}

	var ranges []lineRange
	if n <= headTailLines*2 {
		ranges = append(ranges, lineRange{0, n})
	} else {
		ranges = append(ranges, lineRange{0, headTailLines})
		ranges = append(ranges, lineRange{n - headTailLines, n})
	}
	for i, l := range lines {
		if errorLine.MatchString(l) {
			start := i - errorContextRadius
			if start < 0 {
				start = 0
			}
			end := i + errorContextRadius + 1
			if end > n {
				end = n
			}
			ranges = append(ranges, lineRange{start, end})
		}
	}

	// sort ranges by start (insertion above is already head, tail, then in-order error
	// ranges, so a simple stable sort keeps this deterministic and cheap).
	for i := 1; i < len(ranges); i++ {
		for j := i; j > 0 && ranges[j-1].start > ranges[j].start; j-- {
			ranges[j-1], ranges[j] = ranges[j], ranges[j-1]
		}
	}
	ranges = mergeRanges(ranges)

	var out []string
	prevEnd := 0
	for _, r := range ranges {
		if r.start > prevEnd {
			out = append(out, fmt.Sprintf("[... %d lines omitted ...]", r.start-prevEnd))
		}
		out = append(out, lines[r.start:r.end]...)
		if r.end > prevEnd {
			prevEnd = r.end
		}
	}
	if prevEnd < n {
		out = append(out, fmt.Sprintf("[... %d lines omitted ...]", n-prevEnd))
	}
	return out
}

// tailCutIfNeeded is the "diff" profile's minimal truncation: keep the head of the output
// up to budget and mark the cut, without dedup/error-priority (each diff line is meaningful).
func tailCutIfNeeded(lines []string, budget int) []string {
	if budget <= 0 || totalLen(lines) <= budget {
		return lines
	}
	kept := 0
	total := 0
	for i, l := range lines {
		total += len(l) + 1
		if total > budget {
			kept = i
			break
		}
		kept = i + 1
	}
	out := make([]string, 0, kept+1)
	out = append(out, lines[:kept]...)
	out = append(out, fmt.Sprintf("[... %d lines omitted ...]", len(lines)-kept))
	return out
}

func totalLen(lines []string) int {
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	return total
}
