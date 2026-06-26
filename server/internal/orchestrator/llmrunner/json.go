package llmrunner

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseJSONMarkdown(content string) (map[string]any, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if strings.HasSuffix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	var res map[string]any

	sanitized := SanitizeJSON(trimmed)
	if strings.HasPrefix(sanitized, "[") {
		var arr []any
		if err := json.Unmarshal([]byte(sanitized), &arr); err == nil {
			return map[string]any{"array": arr}, nil
		}
		start := strings.Index(sanitized, "[")
		end := strings.LastIndex(sanitized, "]")
		if start != -1 && end != -1 && end > start {
			extracted := sanitized[start : end+1]
			if err := json.Unmarshal([]byte(extracted), &arr); err == nil {
				return map[string]any{"array": arr}, nil
			}
		}
	}
	if err := json.Unmarshal([]byte(sanitized), &res); err != nil {
		start := strings.Index(sanitized, "{")
		end := strings.LastIndex(sanitized, "}")
		if start != -1 && end != -1 && end > start {
			sanitized = sanitized[start : end+1]
			if err := json.Unmarshal([]byte(sanitized), &res); err == nil {
				return res, nil
			}
		}
		return nil, err
	}
	return res, nil
}

func SanitizeJSON(input string) string {
	var result strings.Builder
	inString := false
	escaped := false

	for _, c := range input {
		if inString {
			if escaped {
				escaped = false
				result.WriteRune(c)
			} else if c == '\\' {
				escaped = true
				result.WriteRune(c)
			} else if c == '"' {
				inString = false
				result.WriteRune(c)
			} else if c == '\t' {
				result.WriteString(`\t`)
			} else if c == '\n' {
				result.WriteString(`\n`)
			} else if c < 0x20 {
				result.WriteString(fmt.Sprintf(`\u%04x`, c))
			} else {
				result.WriteRune(c)
			}
		} else {
			if c == '"' {
				inString = true
			}
			result.WriteRune(c)
		}
	}
	return result.String()
}
