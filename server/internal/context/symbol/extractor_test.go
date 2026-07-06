package symbol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
