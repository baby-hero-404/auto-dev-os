package patch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSearchReplace(t *testing.T) {
	tests := []struct {
		name           string
		patchData      string
		expectedBlocks []EditBlock
	}{
		{
			name: "Standard single block",
			patchData: `
File: server/main.go
<<<<<<< SEARCH
func main() {
	fmt.Println("hello")
}
=======
func main() {
	fmt.Println("world")
}
>>>>>>> REPLACE
`,
			expectedBlocks: []EditBlock{
				{
					Filepath: "server/main.go",
					Search:   "func main() {\n\tfmt.Println(\"hello\")\n}\n",
					Replace:  "func main() {\n\tfmt.Println(\"world\")\n}\n",
				},
			},
		},
		{
			name: "Multi-block parsing",
			patchData: `
File: utils.go
<<<<<<< SEARCH
func add(a, b int) int {
	return a + b
}
=======
func add(a, b int) int {
	return a + b
}
func sub(a, b int) int {
	return a - b
}
>>>>>>> REPLACE

` + "`" + `helper.go` + "`" + `
<<<<<<< SEARCH
var x = 1
=======
var x = 2
>>>>>>> REPLACE
`,
			expectedBlocks: []EditBlock{
				{
					Filepath: "utils.go",
					Search:   "func add(a, b int) int {\n\treturn a + b\n}\n",
					Replace:  "func add(a, b int) int {\n\treturn a + b\n}\nfunc sub(a, b int) int {\n\treturn a - b\n}\n",
				},
				{
					Filepath: "helper.go",
					Search:   "var x = 1\n",
					Replace:  "var x = 2\n",
				},
			},
		},
		{
			name: "File Creation (Empty SEARCH)",
			patchData: `
File: newfile.txt
<<<<<<< SEARCH
=======
hello world
>>>>>>> REPLACE
`,
			expectedBlocks: []EditBlock{
				{
					Filepath: "newfile.txt",
					Search:   "",
					Replace:  "hello world\n",
				},
			},
		},
		{
			name: "File deletion/clearing (Empty REPLACE)",
			patchData: `
file: oldfile.txt
<<<<<<< SEARCH
delete me
=======
>>>>>>> REPLACE
`,
			expectedBlocks: []EditBlock{
				{
					Filepath: "oldfile.txt",
					Search:   "delete me\n",
					Replace:  "",
				},
			},
		},
		{
			name: "File Path Heuristics (different prefixes)",
			patchData: `
File: path1.go
<<<<<<< SEARCH
a
=======
b
>>>>>>> REPLACE

file: path2.go
<<<<<<< SEARCH
c
=======
d
>>>>>>> REPLACE

File:path3.go
<<<<<<< SEARCH
e
=======
f
>>>>>>> REPLACE
`,
			expectedBlocks: []EditBlock{
				{
					Filepath: "path1.go",
					Search:   "a\n",
					Replace:  "b\n",
				},
				{
					Filepath: "path2.go",
					Search:   "c\n",
					Replace:  "d\n",
				},
				{
					Filepath: "path3.go",
					Search:   "e\n",
					Replace:  "f\n",
				},
			},
		},
		{
			name: "Malformed markers (missing replace close)",
			patchData: `
File: malformed.go
<<<<<<< SEARCH
a
=======
b
`,
			expectedBlocks: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseSearchReplace(tt.patchData)
			if len(blocks) != len(tt.expectedBlocks) {
				t.Fatalf("Expected %d blocks, got %d", len(tt.expectedBlocks), len(blocks))
			}
			for i, eb := range tt.expectedBlocks {
				if blocks[i].Filepath != eb.Filepath {
					t.Errorf("Block %d: expected filepath %q, got %q", i, eb.Filepath, blocks[i].Filepath)
				}
				if blocks[i].Search != eb.Search {
					t.Errorf("Block %d: expected search %q, got %q", i, eb.Search, blocks[i].Search)
				}
				if blocks[i].Replace != eb.Replace {
					t.Errorf("Block %d: expected replace %q, got %q", i, eb.Replace, blocks[i].Replace)
				}
			}
		})
	}
}

func TestApplySearchReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.txt"
	fullPath := filepath.Join(dir, filePath)

	err := os.WriteFile(fullPath, []byte("line 1\nline 2\nline 3\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "line 2\n",
			Replace:  "line 2 modified\n",
		},
	}

	err = ApplySearchReplace(blocks, dir)
	if err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}

	content, _ := os.ReadFile(fullPath)
	expected := "line 1\nline 2 modified\nline 3\n"
	if string(content) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(content))
	}
}

func TestApplySearchReplace_AmbiguousFuzzyFallbackNamesTier(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.go"
	fullPath := filepath.Join(dir, filePath)

	// Two lines that only differ from the search block by trailing
	// whitespace, so tier 1 (trailing-whitespace) finds both and must
	// fail fast rather than falling through to a fuzzier tier.
	content := "foo()   \nfoo()  \n"
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{{Filepath: filePath, Search: "foo()\n", Replace: "bar()\n"}}

	err = ApplySearchReplace(blocks, dir)
	if err == nil {
		t.Fatalf("expected an ambiguous-match error, got nil")
	}
	if !strings.Contains(err.Error(), "trailing-whitespace") {
		t.Errorf("expected error to name the tier that found the ambiguity, got: %v", err)
	}
}

func TestApplySearchReplace_NewlineNormalization(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.txt"
	fullPath := filepath.Join(dir, filePath)

	// Write file on disk using Windows CRLF newlines
	err := os.WriteFile(fullPath, []byte("line 1\r\nline 2\r\nline 3\r\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Search block using Unix LF newlines
	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "line 2\n",
			Replace:  "line 2 modified\n",
		},
	}

	err = ApplySearchReplace(blocks, dir)
	if err != nil {
		t.Fatalf("Failed to apply: %v", err)
	}

	content, _ := os.ReadFile(fullPath)
	// ApplySearchReplace normalizes everything to \n, so expected file content should have \n
	expected := "line 1\nline 2 modified\nline 3\n"
	if string(content) != expected {
		t.Errorf("Expected normalized content %q, got %q", expected, string(content))
	}
}

func TestApplySearchReplace_ValidationBubbling(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.txt"
	fullPath := filepath.Join(dir, filePath)

	err := os.WriteFile(fullPath, []byte("line 1\nline 2\nline 3\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// S&R block that doesn't exist
	blocks := []EditBlock{
		{
			Filepath: filePath,
			Search:   "line 5\n",
			Replace:  "line 5 modified\n",
		},
	}

	err = ApplySearchReplace(blocks, dir)
	if err == nil {
		t.Fatalf("Expected error for non-existent search block, got nil")
	}

	if !strings.Contains(err.Error(), "search block not found") {
		t.Errorf("Expected error to contain 'search block not found', got %q", err.Error())
	}
}

func TestApplySearchReplace_EmptySearchOverwriteAndCreate(t *testing.T) {
	dir := t.TempDir()

	// Case 1: Existing file + empty search -> should overwrite content entirely
	existingFilePath := "existing.txt"
	existingFullPath := filepath.Join(dir, existingFilePath)
	err := os.WriteFile(existingFullPath, []byte("original file content\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	blocks := []EditBlock{
		{
			Filepath: existingFilePath,
			Search:   "",
			Replace:  "new overwritten content\n",
		},
	}

	err = ApplySearchReplace(blocks, dir)
	if err != nil {
		t.Fatalf("Failed to apply search_replace overwrite: %v", err)
	}

	content, _ := os.ReadFile(existingFullPath)
	if string(content) != "new overwritten content\n" {
		t.Errorf("Expected file to be overwritten, got %q", string(content))
	}

	// Case 2: Non-existent file + empty search -> should create the file with content
	newFilePath := "newfile.txt"
	newFullPath := filepath.Join(dir, newFilePath)

	blocksNew := []EditBlock{
		{
			Filepath: newFilePath,
			Search:   "",
			Replace:  "created file content\n",
		},
	}

	err = ApplySearchReplace(blocksNew, dir)
	if err != nil {
		t.Fatalf("Failed to apply search_replace create: %v", err)
	}

	newContent, err := os.ReadFile(newFullPath)
	if err != nil {
		t.Fatalf("Expected file to be created: %v", err)
	}
	if string(newContent) != "created file content\n" {
		t.Errorf("Expected created file to have content %q, got %q", "created file content\n", string(newContent))
	}
}
