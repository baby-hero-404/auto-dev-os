package patch

import (
	"strconv"
	"strings"
)

// tierNames maps a fuzzy-fallback tier index (as returned by findMatch) to a
// human-readable name for error messages and telemetry logs.
var tierNames = []string{"exact", "trailing-whitespace", "relative-indent", "line-trim"}

// trailingWSMatch finds a window in content whose lines match search's lines
// after stripping only TRAILING whitespace from each line (leading
// indentation must match exactly). This is tier 1 of the fuzzy fallback
// pipeline: it tolerates LLM output that drops trailing spaces/tabs a
// formatter would otherwise strip, without being as permissive as a full
// per-line trim (which would also mask indentation drift).
func trailingWSMatch(content, search string) (start, end int, deltas []int, indentChar byte, ok bool, matchCount int) {
	searchLines := splitLinesKeepEnds(search)
	if len(searchLines) == 0 {
		return 0, 0, nil, 0, false, 0
	}
	trimmedSearch := make([]string, len(searchLines))
	for i, l := range searchLines {
		trimmedSearch[i] = strings.TrimRight(stripLineEnding(l), " \t")
	}

	contentLines := splitLinesKeepEnds(content)

	var matches [][2]int
	for i := 0; i+len(searchLines) <= len(contentLines); i++ {
		match := true
		for j, ts := range trimmedSearch {
			if strings.TrimRight(stripLineEnding(contentLines[i+j]), " \t") != ts {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, [2]int{i, i + len(searchLines)})
		}
	}

	if len(matches) != 1 {
		return 0, 0, nil, 0, false, len(matches)
	}

	m := matches[0]
	start = lineOffset(contentLines, m[0])
	end = lineOffset(contentLines, m[1])
	window := contentLines[m[0]:m[1]]
	deltas = indentDeltas(searchLines, window)
	indentChar = detectIndentChar(window)
	return start, end, deltas, indentChar, true, 1
}

// trimmedLineMatch finds a window in content whose lines match search's lines
// after per-line whitespace trimming. This tolerates LLM-generated SEARCH
// blocks that differ from the file only by leading/trailing whitespace.
// Returns byte offsets [start, end) in the ORIGINAL content, plus the
// per-line indent delta (file indent length minus search indent length) so
// the caller can reindent the REPLACE text to the file's own indentation
// rather than pasting the LLM's search-block indentation verbatim.
func trimmedLineMatch(content, search string) (start, end int, deltas []int, indentChar byte, ok bool, matchCount int) {
	searchLines := splitLinesKeepEnds(search)
	if len(searchLines) == 0 {
		return 0, 0, nil, 0, false, 0
	}
	trimmedSearch := make([]string, len(searchLines))
	for i, l := range searchLines {
		trimmedSearch[i] = strings.TrimSpace(stripLineEnding(l))
	}

	contentLines := splitLinesKeepEnds(content)

	var matches [][2]int // [startLine, endLineExclusive) in contentLines
	for i := 0; i+len(searchLines) <= len(contentLines); i++ {
		match := true
		for j, ts := range trimmedSearch {
			if strings.TrimSpace(stripLineEnding(contentLines[i+j])) != ts {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, [2]int{i, i + len(searchLines)})
		}
	}

	if len(matches) != 1 {
		return 0, 0, nil, 0, false, len(matches)
	}

	m := matches[0]
	start = lineOffset(contentLines, m[0])
	end = lineOffset(contentLines, m[1])
	window := contentLines[m[0]:m[1]]
	deltas = indentDeltas(searchLines, window)
	indentChar = detectIndentChar(window)
	return start, end, deltas, indentChar, true, 1
}

// relativeIndentMatch compares the search block and candidate file windows
// after re-encoding each line's indentation relative to the previous line's
// indentation, so a uniformly shifted (but internally consistent) indent
// level in the LLM output still matches. Returns byte offsets in the
// ORIGINAL content, plus the per-line indent delta (file indent length minus
// search indent length — constant across the window by construction of the
// relative-indent match) and the file's indent character, so the caller can
// reindent REPLACE to match the file's own indentation rather than pasting
// the LLM's search-block indentation.
func relativeIndentMatch(content, search string) (start, end int, deltas []int, indentChar byte, ok bool, matchCount int) {
	searchLines := splitLinesKeepEnds(search)
	if len(searchLines) == 0 {
		return 0, 0, nil, 0, false, 0
	}
	searchBody := make([]string, len(searchLines))
	for i, l := range searchLines {
		searchBody[i] = stripLineEnding(l)
	}
	searchRel := relativeIndent(searchBody)

	contentLines := splitLinesKeepEnds(content)
	contentBody := make([]string, len(contentLines))
	for i, l := range contentLines {
		contentBody[i] = stripLineEnding(l)
	}

	var matches [][2]int
	for i := 0; i+len(searchBody) <= len(contentBody); i++ {
		window := contentBody[i : i+len(searchBody)]
		if relIndentEqual(relativeIndent(window), searchRel) {
			matches = append(matches, [2]int{i, i + len(searchBody)})
		}
	}

	if len(matches) != 1 {
		return 0, 0, nil, 0, false, len(matches)
	}

	m := matches[0]
	start = lineOffset(contentLines, m[0])
	end = lineOffset(contentLines, m[1])
	window := contentLines[m[0]:m[1]]
	deltas = indentDeltas(searchLines, window)
	indentChar = detectIndentChar(window)
	return start, end, deltas, indentChar, true, 1
}

// indentDeltas returns, for each paired line, the difference in leading
// whitespace length between the matched file line and the search line
// (file indent minus search indent).
func indentDeltas(searchLines, contentLines []string) []int {
	n := min(len(searchLines), len(contentLines))
	deltas := make([]int, n)
	for i := range n {
		deltas[i] = indentLen(stripLineEnding(contentLines[i])) - indentLen(stripLineEnding(searchLines[i]))
	}
	return deltas
}

func indentLen(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// detectIndentChar returns the leading whitespace character used by the
// first indented line in lines (' ' or '\t'), defaulting to ' ' when none of
// the lines are indented.
func detectIndentChar(lines []string) byte {
	for _, l := range lines {
		body := stripLineEnding(l)
		trimmed := strings.TrimLeft(body, " \t")
		if len(trimmed) < len(body) {
			return body[0]
		}
	}
	return ' '
}

// reindentReplace rewrites replace's leading whitespace on each line using
// the per-line indentDeltas computed between the matched search block and
// the file's actual content, so the applied text takes on the file's own
// indentation depth AND character (spaces vs tabs) rather than being
// spliced in verbatim from the LLM output. Lines beyond the search block's
// length (insertions) reuse the last known delta. An empty deltas slice
// leaves replace untouched.
func reindentReplace(replace string, deltas []int, indentChar byte) string {
	if len(deltas) == 0 {
		return replace
	}
	if indentChar == 0 {
		indentChar = ' '
	}
	lines := splitLinesKeepEnds(replace)
	var b strings.Builder
	for i, l := range lines {
		ending := ""
		body := l
		if strings.HasSuffix(l, "\n") {
			ending = "\n"
			body = l[:len(l)-1]
		}
		trimmed := strings.TrimLeft(body, " \t")
		indent := body[:len(body)-len(trimmed)]

		delta := deltas[min(i, len(deltas)-1)]
		newLen := max(len(indent)+delta, 0)

		b.WriteString(strings.Repeat(string(indentChar), newLen))
		b.WriteString(trimmed)
		b.WriteString(ending)
	}
	return b.String()
}

// relativeIndent re-encodes each line as (indentDelta, trimmedContent) where
// indentDelta is the change in leading-whitespace length from the previous
// line. This makes two blocks that differ only by a constant indent offset
// compare as equal.
func relativeIndent(lines []string) []string {
	out := make([]string, len(lines))
	prevIndent := 0
	for i, l := range lines {
		trimmed := strings.TrimLeft(l, " \t")
		indent := len(l) - len(trimmed)
		// The first line's delta is always 0 (relative to itself), not its
		// absolute indent — otherwise two blocks shifted by a constant base
		// indent would never compare equal, defeating the point of this
		// "relative to previous line" encoding.
		var delta int
		if i > 0 {
			delta = indent - prevIndent
		}
		out[i] = strconv.Itoa(delta) + "\x00" + trimmed
		prevIndent = indent
	}
	return out
}

func relIndentEqual(a, b []string) bool {
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

// nearestSimilarRange finds the line range in content most similar to search
// using a token-overlap score, for use in error messages only (never applied).
func nearestSimilarRange(content, search string) (startLine, endLine int, snippet string) {
	searchLines := splitLinesKeepEnds(search)
	if len(searchLines) == 0 {
		return 0, 0, ""
	}
	searchTokens := lineTokenSet(search)

	contentLines := splitLinesKeepEnds(content)
	if len(contentLines) == 0 {
		return 0, 0, ""
	}

	bestScore := -1.0
	bestStart := 0
	windowSize := min(len(searchLines), len(contentLines))
	if windowSize == 0 {
		return 0, 0, ""
	}

	for i := 0; i+windowSize <= len(contentLines); i++ {
		window := strings.Join(contentLines[i:i+windowSize], "")
		score := tokenOverlapScore(searchTokens, lineTokenSet(window))
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}

	if bestScore <= 0 {
		return 0, 0, ""
	}

	snippetLines := contentLines[bestStart : bestStart+windowSize]
	return bestStart + 1, bestStart + windowSize, strings.Join(snippetLines, "")
}

func lineTokenSet(s string) map[string]bool {
	set := make(map[string]bool)
	for f := range strings.FieldsSeq(s) {
		set[f] = true
	}
	return set
}

func tokenOverlapScore(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for t := range a {
		if b[t] {
			shared++
		}
	}
	union := len(a) + len(b) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

// splitLinesKeepEnds splits s into lines, keeping the trailing "\n" on each
// line (except possibly the last), so line offsets can be mapped back to
// byte offsets in the original string via lineOffset.
func splitLinesKeepEnds(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func stripLineEnding(l string) string {
	return strings.TrimSuffix(l, "\n")
}

// lineOffset returns the byte offset of the start of lines[idx] within the
// original string that produced lines via splitLinesKeepEnds.
func lineOffset(lines []string, idx int) int {
	offset := 0
	for i := range idx {
		offset += len(lines[i])
	}
	return offset
}
