package parser

import (
	"embed"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

//go:embed queries/*.scm
var queriesFS embed.FS

// languageID names the tag query file (queries/<languageID>-tags.scm) for a file extension —
// mirrors Aider's own get_scm_fname(lang) (references/aider/aider/repomap.py), which resolves
// "queries/<subdir>/<lang>-tags.scm" by a language name rather than the raw extension. Multiple
// extensions can share one query file (.js/.jsx/.mjs/.cjs all use "javascript"; .ts/.tsx both use
// "typescript", since the tsx grammar is a superset of the typescript one for tagging purposes).
var languageID = map[string]string{
	".go":    "go",
	".py":    "python",
	".java":  "java",
	".js":    "javascript",
	".jsx":   "javascript",
	".mjs":   "javascript",
	".cjs":   "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".c":     "c",
	".h":     "c",
	".cpp":   "cpp",
	".cc":    "cpp",
	".cxx":   "cpp",
	".hpp":   "cpp",
	".hh":    "cpp",
	".hxx":   "cpp",
	".cs":    "csharp",
	".rs":    "rust",
	".php":   "php",
	".swift": "swift",
	".kt":    "kotlin",
	".kts":   "kotlin",
	".scala": "scala",
	".lua":   "lua",
	".elm":   "elm",
	".sh":    "bash",
	".bash":  "bash",
}

var parsers = map[string]*sitter.Language{
	".go":    golang.GetLanguage(),
	".py":    python.GetLanguage(),
	".java":  java.GetLanguage(),
	".js":    javascript.GetLanguage(),
	".jsx":   javascript.GetLanguage(),
	".mjs":   javascript.GetLanguage(),
	".cjs":   javascript.GetLanguage(),
	".ts":    typescript.GetLanguage(),
	".tsx":   tsx.GetLanguage(),
	".c":     c.GetLanguage(),
	".h":     c.GetLanguage(),
	".cpp":   cpp.GetLanguage(),
	".cc":    cpp.GetLanguage(),
	".cxx":   cpp.GetLanguage(),
	".hpp":   cpp.GetLanguage(),
	".hh":    cpp.GetLanguage(),
	".hxx":   cpp.GetLanguage(),
	".cs":    csharp.GetLanguage(),
	".rs":    rust.GetLanguage(),
	".php":   php.GetLanguage(),
	".swift": swift.GetLanguage(),
	".kt":    kotlin.GetLanguage(),
	".kts":   kotlin.GetLanguage(),
	".scala": scala.GetLanguage(),
	".lua":   lua.GetLanguage(),
	".elm":   elm.GetLanguage(),
	".sh":    bash.GetLanguage(),
	".bash":  bash.GetLanguage(),
}

// GetLanguage returns the tree-sitter language parser for the given extension.
func GetLanguage(ext string) *sitter.Language {
	return parsers[ext]
}

// GetQuery returns the .scm tag query for the given extension, read from queries/<lang>-tags.scm
// (see languageID). One file per language — not a Go string literal per case — so adding or
// tuning a language's tags is a query-file change, not a code change, matching how Aider itself
// organizes queries/<subdir>/<lang>-tags.scm rather than inlining them in repomap.py.
func GetQuery(ext string) ([]byte, error) {
	lang, ok := languageID[ext]
	if !ok {
		return nil, fmt.Errorf("no query available for %s", ext)
	}
	return queriesFS.ReadFile(fmt.Sprintf("queries/%s-tags.scm", lang))
}
