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

	if err := json.Unmarshal([]byte(sanitized), &res); err == nil {
		return res, nil
	}

	// Try repairing brackets as a fallback before robust parsing
	repaired := RepairJSONBrackets(sanitized)
	if err := json.Unmarshal([]byte(repaired), &res); err == nil {
		return res, nil
	}

	// Fallback to robust parsing
	robustRes, err := RobustParseLLMResponse(trimmed)
	if err == nil {
		return robustRes, nil
	}

	// If robust parsing also fails, try the { } extraction as a final fallback
	start := strings.Index(sanitized, "{")
	end := strings.LastIndex(sanitized, "}")
	if start != -1 && end != -1 && end > start {
		extracted := sanitized[start : end+1]
		if err := json.Unmarshal([]byte(extracted), &res); err == nil {
			return res, nil
		}
		repairedExtracted := RepairJSONBrackets(extracted)
		if err := json.Unmarshal([]byte(repairedExtracted), &res); err == nil {
			return res, nil
		}
	}

	// Classify the failure and return a ClassifiedParseError
	return nil, ClassifyParseError(content, err)
}

func RobustParseLLMResponse(content string) (map[string]any, error) {
	res := make(map[string]any)
	hasAny := false

	if arr, err := extractArray(content, "files_changed"); err == nil {
		res["files_changed"] = arr
		hasAny = true
	}

	if sum, err := extractString(content, "summary"); err == nil {
		res["summary"] = sum
		hasAny = true
	}

	if pat, err := extractString(content, "patch"); err == nil {
		res["patch"] = pat
		hasAny = true
	} else if pat, err := extractString(content, "patch_text"); err == nil {
		res["patch"] = pat
		hasAny = true
	} else if pat, err := extractString(content, "diff"); err == nil {
		res["patch"] = pat
		hasAny = true
	}

	if !hasAny {
		return nil, fmt.Errorf("robust parsing extracted no known keys")
	}
	return res, nil
}

func extractArray(content string, key string) ([]any, error) {
	idx := strings.Index(content, `"`+key+`"`)
	if idx == -1 {
		idx = strings.Index(content, `'`+key+`'`)
	}
	if idx == -1 {
		idx = strings.Index(content, key)
	}
	if idx == -1 {
		return nil, fmt.Errorf("key %q not found", key)
	}
	sub := content[idx:]
	start := strings.Index(sub, "[")
	if start == -1 {
		return nil, fmt.Errorf("array start not found for %q", key)
	}
	depth := 0
	end := -1
	for i := start; i < len(sub); i++ {
		if sub[i] == '[' {
			depth++
		} else if sub[i] == ']' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end == -1 {
		return nil, fmt.Errorf("array end not found for %q", key)
	}

	var arr []any
	if err := json.Unmarshal([]byte(sub[start:end+1]), &arr); err != nil {
		inner := strings.TrimSpace(sub[start+1 : end])
		if inner == "" {
			return []any{}, nil
		}
		var fallbackArr []any
		parts := strings.Split(inner, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			part = strings.Trim(part, `"'`)
			fallbackArr = append(fallbackArr, part)
		}
		return fallbackArr, nil
	}
	return arr, nil
}

func extractString(content string, key string) (string, error) {
	idx := strings.Index(content, `"`+key+`"`)
	if idx == -1 {
		idx = strings.Index(content, `'`+key+`'`)
	}
	if idx == -1 {
		idx = strings.Index(content, key)
	}
	if idx == -1 {
		return "", fmt.Errorf("key %q not found", key)
	}
	sub := content[idx:]
	colonIdx := strings.Index(sub, ":")
	if colonIdx == -1 {
		return "", fmt.Errorf("colon not found for %q", key)
	}
	sub = sub[colonIdx+1:]
	startQuote := -1
	quoteChar := byte('"')
	for i := 0; i < len(sub); i++ {
		if sub[i] == '"' || sub[i] == '\'' {
			startQuote = i
			quoteChar = sub[i]
			break
		}
	}
	if startQuote == -1 {
		return "", fmt.Errorf("start quote not found for %q", key)
	}
	sub = sub[startQuote+1:]

	endQuoteIdx := -1
	for i := 0; i < len(sub); i++ {
		if sub[i] == quoteChar {
			isEscaped := false
			for j := i - 1; j >= 0; j-- {
				if sub[j] == '\\' {
					isEscaped = !isEscaped
				} else {
					break
				}
			}
			if isEscaped {
				continue
			}

			rest := strings.TrimSpace(sub[i+1:])
			isEnd := false
			if rest == "" || rest == "}" || strings.HasPrefix(rest, "}") {
				isEnd = true
			} else if strings.HasPrefix(rest, ",") {
				restAfterComma := strings.TrimSpace(rest[1:])
				if strings.HasPrefix(restAfterComma, `"`) || strings.HasPrefix(restAfterComma, `'`) {
					nextQuoteChar := restAfterComma[0]
					foundColon := false
					escaped := false
					for k := 1; k < len(restAfterComma); k++ {
						if escaped {
							escaped = false
							continue
						}
						if restAfterComma[k] == '\\' {
							escaped = true
							continue
						}
						if restAfterComma[k] == nextQuoteChar {
							afterKey := strings.TrimSpace(restAfterComma[k+1:])
							if strings.HasPrefix(afterKey, ":") {
								foundColon = true
							}
							break
						}
					}
					if foundColon {
						isEnd = true
					}
				}
			}
			if isEnd {
				endQuoteIdx = i
				break
			}
		}
	}

	if endQuoteIdx == -1 {
		for i := len(sub) - 1; i >= 0; i-- {
			if sub[i] == quoteChar {
				isEscaped := false
				for j := i - 1; j >= 0; j-- {
					if sub[j] == '\\' {
						isEscaped = !isEscaped
					} else {
						break
					}
				}
				if !isEscaped {
					endQuoteIdx = i
					break
				}
			}
		}
	}

	if endQuoteIdx == -1 {
		return "", fmt.Errorf("end quote not found for %q", key)
	}

	rawVal := sub[:endQuoteIdx]
	return unescapeJSONString(rawVal), nil
}

func unescapeJSONString(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				sb.WriteByte('\n')
				i++
			case 't':
				sb.WriteByte('\t')
				i++
			case 'r':
				sb.WriteByte('\r')
				i++
			case 'b':
				sb.WriteByte('\b')
				i++
			case 'f':
				sb.WriteByte('\f')
				i++
			case '"':
				sb.WriteByte('"')
				i++
			case '\'':
				sb.WriteByte('\'')
				i++
			case '\\':
				sb.WriteByte('\\')
				i++
			case '/':
				sb.WriteByte('/')
				i++
			case 'u':
				if i+5 < len(s) {
					var codepoint rune
					_, err := fmt.Sscanf(s[i+2:i+6], "%x", &codepoint)
					if err == nil {
						sb.WriteRune(codepoint)
						i += 5
						continue
					}
				}
				sb.WriteByte('\\')
			default:
				sb.WriteByte('\\')
			}
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
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

func RepairJSONBrackets(input string) string {
	var result strings.Builder
	inString := false
	escaped := false
	var stack []byte

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			result.WriteRune(c)
		} else {
			if c == '"' {
				inString = true
				result.WriteRune(c)
			} else if c == '[' {
				stack = append(stack, '[')
				result.WriteRune(c)
			} else if c == '{' {
				stack = append(stack, '{')
				result.WriteRune(c)
			} else if c == ']' {
				if len(stack) > 0 {
					top := stack[len(stack)-1]
					if top == '[' {
						stack = stack[:len(stack)-1]
						result.WriteRune(c)
					} else if top == '{' {
						result.WriteRune('}')
						stack = stack[:len(stack)-1]
					}
				} else {
					result.WriteRune(c)
				}
			} else if c == '}' {
				if len(stack) > 0 {
					top := stack[len(stack)-1]
					if top == '{' {
						stack = stack[:len(stack)-1]
						result.WriteRune(c)
					} else if top == '[' {
						// Mismatch: array is open but we got a curly brace closing.
						// Repair by closing with square bracket.
						result.WriteRune(']')
						stack = stack[:len(stack)-1]
					}
				} else {
					result.WriteRune(c)
				}
			} else {
				result.WriteRune(c)
			}
		}
	}
	return result.String()
}
