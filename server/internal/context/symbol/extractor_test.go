package symbol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

func TestExtractor(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a Go source file
	goFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(goFile, []byte(`
package main

func MyFunc() {
	OtherCall()
}

type User struct {}
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tags, err := ExtractTags(goFile)
	if err != nil {
		t.Fatal(err)
	}

	hasDef := false
	hasRef := false
	for _, tag := range tags {
		if strings.HasSuffix(tag.Name, "MyFunc") && tag.Kind == "def" {
			hasDef = true
		}
		if tag.Name == "OtherCall" && tag.Kind == "ref" {
			hasRef = true
		}
	}

	if !hasDef {
		t.Error("Missing 'def' tag for MyFunc")
	}
	if !hasRef {
		t.Error("Missing 'ref' tag for OtherCall")
	}
}

// TestExtractor_DefEndLineSpansFunctionBody reproduces the bug traced from task
// e2ccc4fa-8f81-455c-8f91-73380c620b31: the tree-sitter query only captured a definition's name
// identifier (a single-line token), so Tag.Line == Tag.EndLine for every "def" tag regardless of
// how long the function actually was. Two real consumers depended on EndLine being accurate:
// FormatSkeleton's line-count-per-def (if ever added) and, critically, provider.RetrieveContext's
// snippet slicing, which treats EndLine<=Line as "unknown" and falls back to Line+80 (then clamps
// to EOF) — so every "Relevant Code Snippet" shown to the LLM was actually "from the function
// start to the end of the file", not the function's real body.
func TestExtractor_DefEndLineSpansFunctionBody(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "main.go")
	src := `package main

func ShortFunc() {
	OtherCall()
}

func LongFunc() {
	line1()
	line2()
	line3()
	line4()
	line5()
}
`
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tags, err := ExtractTags(goFile)
	if err != nil {
		t.Fatal(err)
	}

	var shortDef, longDef *source.Tag
	for i := range tags {
		if tags[i].Kind != "def" {
			continue
		}
		if strings.HasSuffix(tags[i].Name, "ShortFunc") {
			shortDef = &tags[i]
		}
		if strings.HasSuffix(tags[i].Name, "LongFunc") {
			longDef = &tags[i]
		}
	}

	if shortDef == nil || longDef == nil {
		t.Fatalf("expected both ShortFunc and LongFunc def tags, got: %+v", tags)
	}

	if shortDef.EndLine <= shortDef.Line {
		t.Errorf("ShortFunc: expected EndLine (%d) > Line (%d), span collapsed to the identifier token", shortDef.EndLine, shortDef.Line)
	}
	if longDef.EndLine <= longDef.Line {
		t.Errorf("LongFunc: expected EndLine (%d) > Line (%d), span collapsed to the identifier token", longDef.EndLine, longDef.Line)
	}

	shortSpan := shortDef.EndLine - shortDef.Line
	longSpan := longDef.EndLine - longDef.Line
	if longSpan <= shortSpan {
		t.Errorf("expected LongFunc's span (%d lines) to be larger than ShortFunc's (%d lines) — EndLine isn't tracking real function length", longSpan, shortSpan)
	}
}

// TestExtractor_MultiLanguage covers the languages added to fix the "only Go/Python are
// supported" gap traced in this session: any other extension fell through to
// parser.ExtractFallbackTags, which tags every identifier-looking word in the file as an
// untyped "ref" with Line=-1 and never produces a single "def" — meaning FormatSkeleton (which
// only renders "def" tags) and SearchTags (which only considers "def" tags) both treated those
// files as if they didn't exist at all. Each case checks a definition is found with a real
// (non-collapsed) EndLine span, and at least one reference is found.
func TestExtractor_MultiLanguage(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		src       string
		defSuffix string // tag.Name must end with this
		refName   string // exact tag.Name for a "ref" tag
	}{
		{
			name:      "java method",
			filename:  "Main.java",
			defSuffix: "bar",
			refName:   "baz",
			src: `
class Foo {
  int bar(int x) {
    return baz(x);
  }
}
`,
		},
		{
			name:      "java interface",
			filename:  "Iface.java",
			defSuffix: "Greeter",
			refName:   "baz",
			src: `
interface Greeter {
  void greet();
}
class Impl {
  void run() { baz(); }
}
`,
		},
		{
			name:      "javascript function declaration",
			filename:  "main.js",
			defSuffix: "foo",
			refName:   "bar",
			src: `
function foo() {
  bar();
}
`,
		},
		{
			name:      "javascript arrow function assigned to const",
			filename:  "handler.js",
			defSuffix: "handler",
			refName:   "method",
			src: `
const handler = () => {
  obj.method();
};
`,
		},
		{
			name:      "typescript function with types",
			filename:  "hook.ts",
			defSuffix: "useTaskDetail",
			refName:   "fetchData",
			src: `
export function useTaskDetail(): void {
  fetchData();
}
`,
		},
		{
			name:      "typescript interface",
			filename:  "types.ts",
			defSuffix: "TaskDetailContextType",
			refName:   "helper",
			src: `
export interface TaskDetailContextType {
  taskID: string;
}
function helperCaller() { helper(); }
`,
		},
		{
			name:      "tsx react component",
			filename:  "TaskHeader.tsx",
			defSuffix: "TaskHeader",
			refName:   "useTaskDetail",
			src: `
export function TaskHeader() {
  const data = useTaskDetail();
  return <div>{data}</div>;
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			file := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(file, []byte(tt.src), 0644); err != nil {
				t.Fatal(err)
			}

			tags, err := ExtractTags(file)
			if err != nil {
				t.Fatal(err)
			}

			var def *source.Tag
			hasRef := false
			for i := range tags {
				if tags[i].Kind == "def" && strings.HasSuffix(tags[i].Name, tt.defSuffix) {
					def = &tags[i]
				}
				if tags[i].Kind == "ref" && tags[i].Name == tt.refName {
					hasRef = true
				}
			}

			if def == nil {
				t.Fatalf("missing 'def' tag ending in %q, got: %+v", tt.defSuffix, tags)
			}
			if def.EndLine <= def.Line {
				t.Errorf("def %q: expected EndLine (%d) > Line (%d), span collapsed to the identifier token (fell back to regex extraction?)", def.Name, def.EndLine, def.Line)
			}
			if !hasRef {
				t.Errorf("missing 'ref' tag %q, got: %+v", tt.refName, tags)
			}
		})
	}
}

// TestExtractor_MultiLanguage_AiderAdaptedPatterns covers the patterns adopted from
// references/aider/aider/queries (Aider's own tag-extraction queries — this feature's stated
// inspiration) that a first hand-written pass missed: TS interface method signatures, abstract
// classes/methods, enums, namespaces, and JS's other function-definition idioms
// (assignment-expression and object-literal-shorthand methods) beyond plain
// `function foo() {}`/`const foo = () => {}`.
func TestExtractor_MultiLanguage_AiderAdaptedPatterns(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		src       string
		defSuffix string
		refName   string
	}{
		{
			// Multi-line parameter list so the span genuinely differs from a single-line
			// collapse — a method signature that happens to fit on one line would pass this
			// assertion even if the query still only captured the identifier token.
			name:      "typescript interface method signature (no body)",
			filename:  "repo.ts",
			defSuffix: "find",
			refName:   "",
			src: `
export interface Repo {
  find(
    id: string,
    opts: string
  ): Promise<string>;
}
`,
		},
		{
			name:      "typescript abstract class and method",
			filename:  "base.ts",
			defSuffix: "run",
			refName:   "",
			src: `
export abstract class Base {
  abstract run(
    x: number,
    y: number
  ): void;
}
`,
		},
		{
			name:      "typescript enum",
			filename:  "status.ts",
			defSuffix: "Status",
			refName:   "",
			src: `
export enum Status {
  Todo,
  Done,
}
`,
		},
		{
			name:      "typescript namespace",
			filename:  "utils.ts",
			defSuffix: "Utils",
			refName:   "",
			src: `
namespace Utils {
  export function helper() {}
}
`,
		},
		{
			name:      "javascript assignment-expression method",
			filename:  "assign.js",
			defSuffix: "handler",
			refName:   "baz",
			src: `
foo.handler = () => {
  baz();
};
`,
		},
		{
			name:      "javascript object literal shorthand method",
			filename:  "obj.js",
			defSuffix: "method",
			refName:   "qux",
			src: `
const obj = {
  method() {
    qux();
  },
};
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			file := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(file, []byte(tt.src), 0644); err != nil {
				t.Fatal(err)
			}

			tags, err := ExtractTags(file)
			if err != nil {
				t.Fatal(err)
			}

			var def *source.Tag
			hasRef := tt.refName == ""
			for i := range tags {
				if tags[i].Kind == "def" && strings.HasSuffix(tags[i].Name, tt.defSuffix) {
					def = &tags[i]
				}
				if tags[i].Kind == "ref" && tags[i].Name == tt.refName {
					hasRef = true
				}
			}

			if def == nil {
				t.Fatalf("missing 'def' tag ending in %q, got: %+v", tt.defSuffix, tags)
			}
			if def.EndLine <= def.Line {
				t.Errorf("def %q: expected EndLine (%d) > Line (%d), span collapsed to the identifier token (fell back to regex extraction?)", def.Name, def.EndLine, def.Line)
			}
			if !hasRef {
				t.Errorf("missing 'ref' tag %q, got: %+v", tt.refName, tags)
			}
		})
	}
}

// TestExtractor_ExtendedLanguages covers the 11 languages added to reach parity with the subset
// of Aider's language coverage (references/aider/aider/queries) that go-tree-sitter actually has
// grammar bindings for: C, C++, C#, Rust, PHP, Swift, Kotlin, Scala, Lua, Elm, Bash. Each query
// was adapted from Aider's own but verified against this grammar bundle by dumping real parse
// trees rather than assumed compatible — Lua in particular uses a materially different grammar
// here (function_statement/function_name) than Aider's tree-sitter-language-pack assumes
// (function_declaration), which would have silently matched nothing if copied verbatim.
func TestExtractor_ExtendedLanguages(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		src       string
		defSuffix string
		refName   string
	}{
		{
			name:      "c function",
			filename:  "main.c",
			defSuffix: "bar",
			refName:   "baz",
			src: `
int bar(int x) {
  return baz(x);
}
`,
		},
		{
			name:      "c struct",
			filename:  "point.c",
			defSuffix: "Point",
			refName:   "",
			src: `
struct Point {
  int x;
  int y;
};
`,
		},
		{
			name:      "cpp class method",
			filename:  "foo.cpp",
			defSuffix: "bar",
			refName:   "baz",
			src: `
class Foo {
public:
  int bar(int x) {
    return baz(x);
  }
};
`,
		},
		{
			name:      "csharp method",
			filename:  "Foo.cs",
			defSuffix: "Bar",
			refName:   "Baz",
			src: `
class Foo {
  int Bar(int x) {
    return Baz(x);
  }
}
`,
		},
		{
			name:      "rust function",
			filename:  "main.rs",
			defSuffix: "bar",
			refName:   "baz",
			src: `
fn bar() {
    baz();
}
`,
		},
		{
			name:      "php method",
			filename:  "Foo.php",
			defSuffix: "bar",
			refName:   "baz",
			src: `<?php
class Foo {
  function bar($x) {
    return baz($x);
  }
}
`,
		},
		{
			name:      "swift function",
			filename:  "Foo.swift",
			defSuffix: "bar",
			refName:   "baz",
			src: `
func bar(x: Int) -> Int {
  return baz(x)
}
`,
		},
		{
			name:      "kotlin function",
			filename:  "Foo.kt",
			defSuffix: "bar",
			refName:   "baz",
			src: `
fun bar(x: Int): Int {
  return baz(x)
}
`,
		},
		{
			name:      "scala function",
			filename:  "Foo.scala",
			defSuffix: "bar",
			refName:   "baz",
			src: `
def bar(x: Int): Int = {
  baz(x)
}
`,
		},
		{
			name:      "lua function",
			filename:  "foo.lua",
			defSuffix: "bar",
			refName:   "baz",
			src: `
function bar()
  baz()
end
`,
		},
		{
			name:      "lua local function",
			filename:  "local.lua",
			defSuffix: "bar",
			refName:   "baz",
			src: `
local function bar()
  baz()
end
`,
		},
		{
			name:      "bash function",
			filename:  "script.sh",
			defSuffix: "bar",
			refName:   "baz",
			src: `
bar() {
  baz
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			file := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(file, []byte(tt.src), 0644); err != nil {
				t.Fatal(err)
			}

			tags, err := ExtractTags(file)
			if err != nil {
				t.Fatal(err)
			}

			var def *source.Tag
			hasRef := tt.refName == ""
			for i := range tags {
				if tags[i].Kind == "def" && strings.HasSuffix(tags[i].Name, tt.defSuffix) {
					def = &tags[i]
				}
				if tags[i].Kind == "ref" && tags[i].Name == tt.refName {
					hasRef = true
				}
			}

			if def == nil {
				t.Fatalf("missing 'def' tag ending in %q, got: %+v", tt.defSuffix, tags)
			}
			if def.EndLine <= def.Line {
				t.Errorf("def %q: expected EndLine (%d) > Line (%d), span collapsed to the identifier token (fell back to regex extraction?)", def.Name, def.EndLine, def.Line)
			}
			if !hasRef {
				t.Errorf("missing 'ref' tag %q, got: %+v", tt.refName, tags)
			}
		})
	}
}

// TestExtractor_Elm is separate from TestExtractor_ExtendedLanguages because Elm's module-level
// function syntax doesn't produce a multi-line EndLine span the same way C-family braces do
// (`foo x = bar x` is legitimately one line for a trivial body), so it can't reuse that table's
// "EndLine > Line" assertion without a contrived multi-line example.
func TestExtractor_Elm(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "Utils.elm")
	src := `module Utils exposing (bar)

bar : Int -> Int
bar x =
    baz x
`
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tags, err := ExtractTags(file)
	if err != nil {
		t.Fatal(err)
	}

	hasDef, hasModule, hasRef := false, false, false
	for _, tag := range tags {
		if tag.Kind == "def" && strings.HasSuffix(tag.Name, "bar") {
			hasDef = true
		}
		if tag.Kind == "def" && strings.HasSuffix(tag.Name, "Utils") {
			hasModule = true
		}
		if tag.Kind == "ref" && tag.Name == "baz" {
			hasRef = true
		}
	}

	if !hasDef {
		t.Errorf("missing 'def' tag for bar, got: %+v", tags)
	}
	if !hasModule {
		t.Errorf("missing 'def' tag for module Utils, got: %+v", tags)
	}
	if !hasRef {
		t.Errorf("missing 'ref' tag for baz, got: %+v", tags)
	}
}

func TestFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an unsupported file type to trigger regex fallback
	txtFile := filepath.Join(tmpDir, "test.unknown_lang")
	os.WriteFile(txtFile, []byte("HelloWorld SomeFunctionCall_123"), 0644)

	tags, err := ExtractTags(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	hasRef := false
	for _, tag := range tags {
		if tag.Name == "SomeFunctionCall_123" && tag.Kind == "ref" {
			hasRef = true
		}
	}

	if !hasRef {
		t.Error("Fallback lexer failed to extract blind references")
	}
}
