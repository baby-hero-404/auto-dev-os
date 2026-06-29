package patch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSearchReplace(t *testing.T) {
	patchData := `
Here is my patch:

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
`
	blocks := ParseSearchReplace(patchData)
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Filepath != "server/main.go" {
		t.Errorf("Expected filepath 'server/main.go', got '%s'", blocks[0].Filepath)
	}

	expectedSearch := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
	if blocks[0].Search != expectedSearch {
		t.Errorf("Expected search %q, got %q", expectedSearch, blocks[0].Search)
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
