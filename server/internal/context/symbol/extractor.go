package symbol

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/parser"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
	sitter "github.com/smacker/go-tree-sitter"
)

// ExtractTags parses a file and extracts all definitions and references.
func ExtractTags(filepathStr string) ([]source.Tag, error) {
	ext := filepath.Ext(filepathStr)

	lang := parser.GetLanguage(ext)
	if lang == nil {
		// Fallback for unsupported languages
		return parser.ExtractFallbackTags(filepathStr)
	}

	queryBytes, err := parser.GetQuery(ext)
	if err != nil {
		return parser.ExtractFallbackTags(filepathStr)
	}

	content, err := os.ReadFile(filepathStr)
	if err != nil {
		return nil, err
	}

	p := sitter.NewParser()
	p.SetLanguage(lang)

	tree, err := p.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}

	q, err := sitter.NewQuery(queryBytes, lang)
	if err != nil {
		return nil, err
	}

	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	var tags []source.Tag
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		// A definition match carries two captures: "name.definition.*" (the identifier,
		// used for the tag's Name) and "definition.*" (the enclosing declaration node, used
		// for the tag's Line/EndLine span so it reflects the whole function/method/type body
		// instead of collapsing to the single-line identifier token). A reference match only
		// ever has the single "name.reference.*" capture.
		var nameNode *sitter.Node
		var nameTagName string
		var spanNode *sitter.Node

		for _, c := range m.Captures {
			tagName := q.CaptureNameForId(c.Index)
			if strings.HasPrefix(tagName, "name.") {
				nameNode = c.Node
				nameTagName = tagName
			} else if strings.HasPrefix(tagName, "definition.") {
				spanNode = c.Node
			}
		}
		if nameNode == nil {
			continue
		}

		kind := "ref"
		if strings.Contains(nameTagName, "definition") {
			kind = "def"
		}

		rangeNode := nameNode
		if spanNode != nil {
			rangeNode = spanNode
		}

		nodeName := nameNode.Content(content)
		name := nodeName
		if kind == "def" {
			dir := filepath.Base(filepath.Dir(filepathStr))
			file := filepath.Base(filepathStr)
			name = filepath.Join(dir, file) + ": " + nodeName
		}

		tags = append(tags, source.Tag{
			Name:     name,
			Kind:     kind,
			Line:     int(rangeNode.StartPoint().Row),
			EndLine:  int(rangeNode.EndPoint().Row),
			Filepath: filepathStr,
		})
	}

	// If the parser returned an empty result, use fallback to catch unseen refs
	if len(tags) == 0 {
		return parser.ExtractFallbackTags(filepathStr)
	}

	return tags, nil
}
