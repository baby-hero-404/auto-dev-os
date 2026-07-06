package parser

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
)

var parsers = map[string]*sitter.Language{
	".go": golang.GetLanguage(),
	".py": python.GetLanguage(),
}

// GetLanguage returns the tree-sitter language parser for the given extension.
func GetLanguage(ext string) *sitter.Language {
	return parsers[ext]
}

// GetQuery returns the .scm syntax query to extract defs and refs.
func GetQuery(ext string) ([]byte, error) {
	switch ext {
	case ".go":
		// MVP hardcoded query to avoid path resolution issues across runtime environments
		return []byte(`
(function_declaration name: (identifier) @name.definition.function)
(method_declaration name: (field_identifier) @name.definition.method)
(type_declaration (type_spec name: (type_identifier) @name.definition.class))
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (selector_expression field: (field_identifier) @name.reference.call))
`), nil
	case ".py":
		return []byte(`
(function_definition name: (identifier) @name.definition.function)
(class_definition name: (identifier) @name.definition.class)
(call function: (identifier) @name.reference.call)
(call function: (attribute attribute: (identifier) @name.reference.call))
`), nil
	default:
		return nil, fmt.Errorf("no query available for %s", ext)
	}
}
