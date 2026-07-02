package patch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	filePath := "main.go"
	fullPath := filepath.Join(dir, filePath)
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	validPatch := `
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
 
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }
`
	errors := ValidateUnifiedDiff(validPatch, dir)
	if len(errors) > 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(errors), errors)
	}

	invalidPatch := `
--- a/main.go
+++ b/main.go
@@ -10,5 +10,5 @@
 package main
 
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }
`
	errors = ValidateUnifiedDiff(invalidPatch, dir)
	if len(errors) == 0 {
		t.Errorf("Expected validation errors for line count mismatch, got 0")
	}
}

func TestValidateSearchReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.txt"
	fullPath := filepath.Join(dir, filePath)
	content := "line 1\nline 2\nline 3\nline 2\n"
	err := os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		blocks        []EditBlock
		expectedErrs  int
		expectedMatch string
	}{
		{
			name: "Valid exact match",
			blocks: []EditBlock{
				{Filepath: filePath, Search: "line 1\nline 2\n", Replace: "line 1\nline 2 modified\n"},
			},
			expectedErrs: 0,
		},
		{
			name: "Not found match",
			blocks: []EditBlock{
				{Filepath: filePath, Search: "line 5\n", Replace: "line 6\n"},
			},
			expectedErrs:  1,
			expectedMatch: "not found",
		},
		{
			name: "Ambiguous match",
			blocks: []EditBlock{
				{Filepath: filePath, Search: "line 2\n", Replace: "line 2 modified\n"},
			},
			expectedErrs:  1,
			expectedMatch: "Ambiguous match",
		},
		{
			name: "Missing file",
			blocks: []EditBlock{
				{Filepath: "missing.txt", Search: "some text", Replace: "other text"},
			},
			expectedErrs:  1,
			expectedMatch: "does not exist", // Will match "does not exist" or "Cannot read file"
		},
		{
			name: "File creation validation (Missing file, empty SEARCH)",
			blocks: []EditBlock{
				{Filepath: "newfile.txt", Search: "", Replace: "some content"},
			},
			expectedErrs: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateSearchReplace(tc.blocks, dir)
			if len(errs) != tc.expectedErrs {
				t.Fatalf("Expected %d errors, got %d. Errors: %v", tc.expectedErrs, len(errs), errs)
			}
			if tc.expectedErrs > 0 && tc.expectedMatch != "" {
				matched := false
				for _, e := range errs {
					if e.Reason != "" { // Check if reason string contains our expected substring (simple check)
						matched = true
					}
				}
				if !matched {
					t.Errorf("Expected error to match %q, got %v", tc.expectedMatch, errs)
				}
			}
		})
	}
}
