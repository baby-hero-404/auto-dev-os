package llmrunner

import "fmt"

// ParseErrorKind classifies the type of parse failure for targeted retry.
type ParseErrorKind string

const (
	// ParseFormatError means the JSON syntax is invalid (missing quotes, commas, etc.)
	ParseFormatError ParseErrorKind = "format"
	// ParseTruncationError means the LLM response was cut off mid-JSON
	ParseTruncationError ParseErrorKind = "truncation"
	// ParseSchemaError means valid JSON but required keys are missing
	ParseSchemaError ParseErrorKind = "schema"
	// ParseBusinessError means valid JSON/schema but content fails domain validation
	ParseBusinessError ParseErrorKind = "business"
)

// ClassifiedParseError wraps a parse failure with a classification.
type ClassifiedParseError struct {
	Kind    ParseErrorKind
	Message string
	Raw     string // original LLM output
}

func (e *ClassifiedParseError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

// ClassifyParseError analyzes a failed LLM response and classifies the error type.
func ClassifyParseError(content string, parseErr error) *ClassifiedParseError {
	if content == "" {
		return &ClassifiedParseError{
			Kind:    ParseTruncationError,
			Message: "empty response from LLM",
			Raw:     content,
		}
	}

	// Truncation detection: response doesn't end with a closing bracket
	trimmed := trimTrailingWhitespace(content)
	if len(trimmed) > 0 {
		lastChar := trimmed[len(trimmed)-1]
		if lastChar != '}' && lastChar != ']' && lastChar != '`' {
			return &ClassifiedParseError{
				Kind:    ParseTruncationError,
				Message: fmt.Sprintf("response appears truncated (ends with %q): %v", string(lastChar), parseErr),
				Raw:     content,
			}
		}
	}

	// Check bracket depth — unclosed brackets indicate truncation
	depth := bracketDepth(content)
	if depth > 0 {
		return &ClassifiedParseError{
			Kind:    ParseTruncationError,
			Message: fmt.Sprintf("unclosed brackets (depth=%d): %v", depth, parseErr),
			Raw:     content,
		}
	}

	// Format error: JSON syntax issues
	return &ClassifiedParseError{
		Kind:    ParseFormatError,
		Message: fmt.Sprintf("invalid JSON syntax: %v", parseErr),
		Raw:     content,
	}
}

func trimTrailingWhitespace(s string) string {
	i := len(s) - 1
	for i >= 0 && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t') {
		i--
	}
	if i < 0 {
		return ""
	}
	return s[:i+1]
}

func bracketDepth(s string) int {
	depth := 0
	inString := false
	escaped := false
	for _, c := range s {
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
		} else {
			if c == '"' {
				inString = true
			} else if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				depth--
			}
		}
	}
	return depth
}
