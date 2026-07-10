package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// FindSymbolTool implements tool.Tool to search for Go symbols via AST parsing.
type FindSymbolTool struct{}

// Name returns the unique tool name.
func (t *FindSymbolTool) Name() string { return "find_symbol" }

// Category returns the tool's category.
func (t *FindSymbolTool) Category() tool.Category { return tool.CategorySearch }

// Capabilities returns the capability permissions required.
func (t *FindSymbolTool) Capabilities() []tool.Capability { return []tool.Capability{tool.CapSearch} }

// Description returns a description for the LLM.
func (t *FindSymbolTool) Description() string {
	return "Search the workspace for Go symbols (structs, interfaces, functions, methods, variables, constants, type aliases) matching a query."
}

// Schema returns the JSON schema for tool inputs.
func (t *FindSymbolTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "description": "Symbol name or substring to search for"},
			"path":  {"type": "string", "description": "Optional relative path within the workspace to restrict search"}
		}
	}`)
}

type FindSymbolArgs struct {
	Query string `json:"query"`
	Path  string `json:"path"`
}

type SymbolResult struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Signature string `json:"signature"`
}

// Execute runs the symbol search operation.
func (t *FindSymbolTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	var args FindSymbolArgs
	argsBytes, err := json.Marshal(call.Input)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to serialize inputs: %w", err)
	}
	if err := json.Unmarshal(argsBytes, &args); err != nil {
		return tool.Result{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Query == "" {
		return tool.Result{Success: false, Message: "missing required 'query' parameter"}, nil
	}

	searchRoot := call.Workspace
	if args.Path != "" {
		searchRoot, err = tool.SafeWorkspacePath(call.Workspace, args.Path)
		if err != nil {
			return tool.Result{Success: false, Message: fmt.Sprintf("invalid path: %v", err)}, nil
		}
	}

	info, err := os.Stat(searchRoot)
	if err != nil {
		return tool.Result{Success: false, Message: fmt.Sprintf("failed to access path: %v", err)}, nil
	}

	var goFiles []string
	if !info.IsDir() {
		if strings.HasSuffix(searchRoot, ".go") {
			goFiles = append(goFiles, searchRoot)
		}
	} else {
		err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if (strings.HasPrefix(name, ".") && name != ".") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".go") {
				goFiles = append(goFiles, path)
			}
			return nil
		})
		if err != nil {
			return tool.Result{Success: false, Message: fmt.Sprintf("failed to walk directory: %v", err)}, nil
		}
	}

	var matches []SymbolResult
	queryLower := strings.ToLower(args.Query)

	for _, file := range goFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		relFile, _ := filepath.Rel(call.Workspace, file)

		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if strings.Contains(strings.ToLower(name), queryLower) {
					kind := "function"
					var recv string
					if d.Recv != nil && len(d.Recv.List) > 0 {
						kind = "method"
						t := d.Recv.List[0].Type
						switch rt := t.(type) {
						case *ast.Ident:
							recv = rt.Name
						case *ast.StarExpr:
							if ident, ok := rt.X.(*ast.Ident); ok {
								recv = "*" + ident.Name
							}
						}
					}

					var sig string
					if recv != "" {
						sig = fmt.Sprintf("func (%s) %s", recv, name)
					} else {
						sig = fmt.Sprintf("func %s", name)
					}

					matches = append(matches, SymbolResult{
						Name:      name,
						Kind:      kind,
						File:      relFile,
						Line:      fset.Position(d.Pos()).Line,
						Signature: sig,
					})
				}

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						name := s.Name.Name
						if strings.Contains(strings.ToLower(name), queryLower) {
							kind := "type"
							switch s.Type.(type) {
							case *ast.StructType:
								kind = "struct"
							case *ast.InterfaceType:
								kind = "interface"
							}
							matches = append(matches, SymbolResult{
								Name:      name,
								Kind:      kind,
								File:      relFile,
								Line:      fset.Position(s.Pos()).Line,
								Signature: fmt.Sprintf("type %s", name),
							})
						}

					case *ast.ValueSpec:
						kind := "variable"
						if d.Tok == token.CONST {
							kind = "constant"
						}
						for _, nameIdent := range s.Names {
							name := nameIdent.Name
							if strings.Contains(strings.ToLower(name), queryLower) {
								matches = append(matches, SymbolResult{
									Name:      name,
									Kind:      kind,
									File:      relFile,
									Line:      fset.Position(nameIdent.Pos()).Line,
									Signature: fmt.Sprintf("%s %s", d.Tok.String(), name),
								})
							}
						}
					}
				}
			}
		}
	}

	outputBytes, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to format output: %w", err)
	}

	return tool.Result{
		Success: true,
		Output:  string(outputBytes),
		Metadata: map[string]any{
			"match_count": len(matches),
		},
	}, nil
}
