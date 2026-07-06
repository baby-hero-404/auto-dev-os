package symbol

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/auto-code-os/auto-code-os/server/internal/context/parser"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
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
		
		for _, c := range m.Captures {
			tagName := q.CaptureNameForId(c.Index)
			nodeName := c.Node.Content(content)
			
			// Resolve kind based on tree-sitter tag name
			kind := "ref"
			if strings.Contains(tagName, "definition") {
				kind = "def"
			}
			
			name := nodeName
			if kind == "def" {
				dir := filepath.Base(filepath.Dir(filepathStr))
				file := filepath.Base(filepathStr)
				name = filepath.Join(dir, file) + ": " + nodeName
			}
			
			tags = append(tags, source.Tag{
				Name:     name,
				Kind:     kind,
				Line:     int(c.Node.StartPoint().Row),
				EndLine:  int(c.Node.EndPoint().Row),
				Filepath: filepathStr,
			})
		}
	}
	
	// If the parser returned an empty result, use fallback to catch unseen refs
	if len(tags) == 0 {
		return parser.ExtractFallbackTags(filepathStr)
	}

	return tags, nil
}
