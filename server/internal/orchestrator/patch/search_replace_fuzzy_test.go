package patch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrimmedLineMatch(t *testing.T) {
	t.Run("single match with trailing-space-only diff", func(t *testing.T) {
		content := "func main() {\n\tfmt.Println(\"hi\")   \n}\n"
		search := "func main() {\n\tfmt.Println(\"hi\")\n}\n"

		start, end, deltas, indentChar, ok, ambiguous := trimmedLineMatch(content, search)
		if ambiguous {
			t.Fatalf("expected unambiguous match")
		}
		if !ok {
			t.Fatalf("expected match, got none")
		}
		if content[start:end] != "func main() {\n\tfmt.Println(\"hi\")   \n}\n" {
			t.Errorf("matched window = %q", content[start:end])
		}
		if len(deltas) != 3 {
			t.Errorf("expected 3 deltas, got %d: %v", len(deltas), deltas)
		}
		if indentChar != '\t' {
			t.Errorf("expected detected indent char '\\t', got %q", indentChar)
		}
	})

	t.Run("ambiguous when two identical trimmed candidates exist", func(t *testing.T) {
		content := "  x\ny\n  x\ny\n"
		search := "x\ny\n"

		_, _, _, _, ok, ambiguous := trimmedLineMatch(content, search)
		if ok {
			t.Errorf("expected no confirmed match on ambiguous input")
		}
		if !ambiguous {
			t.Errorf("expected ambiguous=true")
		}
	})

	t.Run("no match", func(t *testing.T) {
		content := "foo\nbar\n"
		search := "baz\n"
		_, _, _, _, ok, ambiguous := trimmedLineMatch(content, search)
		if ok || ambiguous {
			t.Errorf("expected no match, no ambiguity; got ok=%v ambiguous=%v", ok, ambiguous)
		}
	})
}

func TestRelativeIndentMatch(t *testing.T) {
	t.Run("uniformly shifted indent level matches", func(t *testing.T) {
		// File nests the block one tab deeper than the LLM's SEARCH block
		// (which omits the enclosing function), but both use tabs uniformly
		// so the relative indent structure lines up.
		content := "func f() {\n\tif true {\n\t\tdoStuff()\n\t}\n}\n"
		search := "if true {\n\tdoStuff()\n}\n"

		start, end, deltas, indentChar, ok, ambiguous := relativeIndentMatch(content, search)
		if ambiguous {
			t.Fatalf("expected unambiguous match")
		}
		if !ok {
			t.Fatalf("expected match, got none")
		}
		if content[start:end] != "\tif true {\n\t\tdoStuff()\n\t}\n" {
			t.Errorf("matched window = %q", content[start:end])
		}
		if len(deltas) == 0 {
			t.Fatalf("expected non-empty deltas")
		}
		for _, d := range deltas {
			if d != deltas[0] {
				t.Errorf("expected constant delta across relative-indent match, got %v", deltas)
			}
		}
		if indentChar != '\t' {
			t.Errorf("expected detected indent char '\\t', got %q", indentChar)
		}
	})

	t.Run("ambiguous relative-indent match returns no partial apply", func(t *testing.T) {
		content := "  a\n    b\n  a\n    b\n"
		search := "a\n  b\n"

		_, _, _, _, ok, ambiguous := relativeIndentMatch(content, search)
		if ok {
			t.Errorf("expected no confirmed match on ambiguous input")
		}
		if !ambiguous {
			t.Errorf("expected ambiguous=true")
		}
	})
}

func TestReindentReplace(t *testing.T) {
	t.Run("shifts replace indentation by the matched delta", func(t *testing.T) {
		deltas := []int{2, 2, 2}
		replace := "if true {\n  doOther()\n}\n"
		got := reindentReplace(replace, deltas, ' ')
		want := "  if true {\n    doOther()\n  }\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("extends the last delta to lines beyond the search block", func(t *testing.T) {
		deltas := []int{1}
		replace := "a\nb\nc\n"
		got := reindentReplace(replace, deltas, ' ')
		want := " a\n b\n c\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("uses the file's indent character, not the LLM's", func(t *testing.T) {
		deltas := []int{1, 1}
		replace := "a\nb\n"
		got := reindentReplace(replace, deltas, '\t')
		want := "\ta\n\tb\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty deltas leaves replace untouched", func(t *testing.T) {
		replace := "  a\nb\n"
		got := reindentReplace(replace, nil, ' ')
		if got != replace {
			t.Errorf("got %q, want unchanged %q", got, replace)
		}
	})
}

func TestNearestSimilarRange(t *testing.T) {
	content := "func alpha() {\n\treturn 1\n}\n\nfunc beta() {\n\treturn 2\n}\n"
	search := "func alpha() {\n\treturn 99\n}\n"

	startLine, endLine, snippet := nearestSimilarRange(content, search)
	if snippet == "" {
		t.Fatalf("expected a non-empty snippet")
	}
	if startLine != 1 || endLine != 3 {
		t.Errorf("expected closest match at lines 1-3, got %d-%d", startLine, endLine)
	}
}

func TestApplySearchReplace_TrimmedWhitespaceFallback_ReindentsToFile(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.go"
	fullPath := filepath.Join(dir, filePath)

	// File has trailing whitespace on line 2 that the LLM's SEARCH block lacks.
	original := "func main() {\n\tfmt.Println(\"hi\")   \n}\n"
	if err := os.WriteFile(fullPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "func main() {\n\tfmt.Println(\"hi\")\n}\n",
			Replace:  "func main() {\n\tfmt.Println(\"bye\")\n}\n",
		},
	}

	if err := ApplySearchReplace(blocks, dir); err != nil {
		t.Fatalf("ApplySearchReplace failed: %v", err)
	}

	content, _ := os.ReadFile(fullPath)
	expected := "func main() {\n\tfmt.Println(\"bye\")\n}\n"
	if string(content) != expected {
		t.Errorf("expected %q, got %q", expected, string(content))
	}
}

func TestApplySearchReplace_RelativeIndentFallback_ReindentsToFile(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.go"
	fullPath := filepath.Join(dir, filePath)

	// File nests the block one tab deeper than the LLM's SEARCH/REPLACE,
	// which omits the enclosing function.
	original := "func f() {\n\tif true {\n\t\tdoStuff()\n\t}\n}\n"
	if err := os.WriteFile(fullPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "if true {\n\tdoStuff()\n}\n",
			Replace:  "if true {\n\tdoOther()\n}\n",
		},
	}

	if err := ApplySearchReplace(blocks, dir); err != nil {
		t.Fatalf("ApplySearchReplace failed: %v", err)
	}

	content, _ := os.ReadFile(fullPath)
	expected := "func f() {\n\tif true {\n\t\tdoOther()\n\t}\n}\n"
	if string(content) != expected {
		t.Errorf("expected file to preserve tab indentation:\nexpected %q\ngot      %q", expected, string(content))
	}
}

func TestApplySearchReplace_SimilarityHintOnTotalFailure(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.go"
	fullPath := filepath.Join(dir, filePath)

	original := "func alpha() {\n\treturn 1\n}\n"
	if err := os.WriteFile(fullPath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "func alpha() {\n\treturn 42\n}\n",
			Replace:  "func alpha() {\n\treturn 42\n}\n",
		},
	}

	err := ApplySearchReplace(blocks, dir)
	if err == nil {
		t.Fatalf("expected error for search block not found anywhere in file")
	}
	if !strings.Contains(err.Error(), "search block not found") || !strings.Contains(err.Error(), "closest match is lines") {
		t.Errorf("expected error to include a similarity hint, got: %v", err)
	}
}
