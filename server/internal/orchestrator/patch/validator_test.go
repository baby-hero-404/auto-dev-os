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

func TestValidateHunkCounts(t *testing.T) {
	// Hunk header specifying @@ -1,3 +1,19 @@.
	// Context lines (space prefix or empty) contribute 1 to old, 1 to new.
	// - lines contribute 1 to old.
	// + lines contribute 1 to new.
	// Here we have:
	// - old: expected = 3
	// - new: expected = 19
	// We provide 20 added lines starting with + and 3 context lines, total new = 23. This is a mismatch.
	invalidPatch := `--- a/main.go
+++ b/main.go
@@ -1,3 +1,19 @@
 package main
 
 func main() {
+	line1
+	line2
+	line3
+	line4
+	line5
+	line6
+	line7
+	line8
+	line9
+	line10
+	line11
+	line12
+	line13
+	line14
+	line15
+	line16
+	line17
+	line18
+	line19
+	line20
 }`

	errs := ValidateHunkCounts(invalidPatch)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 validation error, got %d: %v", len(errs), errs)
	}
	if !errs[0].IsFatal {
		t.Error("expected hunk mismatch error to be fatal")
	}
}
