package steps

import (
	"strings"
	"testing"
)

func TestCompressErrorText_Short(t *testing.T) {
	shortErr := "line 1\nline 2\nline 3"
	compressed := compressErrorText(shortErr)
	if compressed != shortErr {
		t.Errorf("expected short error to be unchanged, got:\n%s", compressed)
	}
}

func TestCompressErrorText_Long(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 150; i++ {
		sb.WriteString("line ")
		sb.WriteString(string(rune('0' + (i % 10))))
		sb.WriteString("\n")
	}
	longErr := strings.TrimSpace(sb.String())

	compressed := compressErrorText(longErr)

	if !strings.Contains(compressed, "... [TRUNCATED: 50 lines omitted for brevity] ...") {
		t.Errorf("expected compressed error to contain truncation marker, got:\n%s", compressed)
	}

	lines := strings.Split(compressed, "\n")
	if len(lines) < 101 { // 20 + marker + 80 + newlines
		t.Errorf("expected at least 101 lines in compressed output, got %d", len(lines))
	}
}
