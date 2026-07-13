package tool

import (
	"encoding/json"
	"testing"
)

func TestToolsForRole(t *testing.T) {
	r := NewRegistry()

	// Register some test tools
	r.Register(&mockTool{name: "read_file", desc: "read", schema: json.RawMessage(`{}`), cat: CategoryFilesystem, caps: []Capability{CapRead}})
	r.Register(&mockTool{name: "edit_file", desc: "edit", schema: json.RawMessage(`{}`), cat: CategoryEditing, caps: []Capability{CapEdit}})
	r.Register(&mockTool{name: "grep_search", desc: "search", schema: json.RawMessage(`{}`), cat: CategorySearch, caps: []Capability{CapSearch}})
	r.Register(&mockTool{name: "generate_docs", desc: "docs", schema: json.RawMessage(`{}`), cat: CategoryDocumentation, caps: []Capability{CapDocs}})

	cm := NewCapabilityManager(r, DefaultRoleProfiles())

	// Test backend role
	beTools := cm.ToolsForRole("backend")
	hasEdit := false
	hasDocs := false
	for _, tool := range beTools {
		if tool.Name == "edit_file" {
			hasEdit = true
		}
		if tool.Name == "generate_docs" {
			hasDocs = true
		}
	}
	if !hasEdit {
		t.Errorf("Expected backend role to have edit tool")
	}
	if hasDocs {
		t.Errorf("Expected backend role to NOT have docs tool")
	}

	// Test reviewer role
	revTools := cm.ToolsForRole("reviewer")
	hasEdit = false
	for _, tool := range revTools {
		if tool.Name == "edit_file" {
			hasEdit = true
		}
	}
	if hasEdit {
		t.Errorf("Expected reviewer role to NOT have edit tool")
	}

	// Test unknown role (fallback to read + search)
	unkTools := cm.ToolsForRole("unknown_role")
	if len(unkTools) != 2 {
		t.Errorf("Expected unknown role to fall back to 2 tools (read + search), got %d", len(unkTools))
	}
	hasRead := false
	hasSearch := false
	for _, tool := range unkTools {
		if tool.Name == "read_file" {
			hasRead = true
		}
		if tool.Name == "grep_search" {
			hasSearch = true
		}
	}
	if !hasRead || !hasSearch {
		t.Errorf("Expected unknown role fallback to include read_file and grep_search")
	}
}

func TestAllowedForRole(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		caps  []Capability
		allow bool
	}{
		{"reviewer rejected for edit tool", "reviewer", []Capability{CapEdit}, false},
		{"reviewer allowed for read tool", "reviewer", []Capability{CapRead}, true},
		{"reviewer allowed for search tool", "reviewer", []Capability{CapSearch}, true},
		{"reviewer allowed for git.diff tool", "reviewer", []Capability{CapGitDiff}, true},
		{"backend allowed for edit tool", "backend", []Capability{CapEdit}, true},
		{"backend rejected for dependency tool", "backend", []Capability{CapDependency}, false},
		{"unknown role falls back to read", "some-made-up-role", []Capability{CapRead}, true},
		{"unknown role rejected for edit", "some-made-up-role", []Capability{CapEdit}, false},
		{"empty role falls back to search", "", []Capability{CapSearch}, true},
		{"role match is case-insensitive", "REVIEWER", []Capability{CapRead}, true},
		{"tool with multiple caps allowed if any match", "reviewer", []Capability{CapEdit, CapRead}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AllowedForRole(tc.role, tc.caps); got != tc.allow {
				t.Errorf("AllowedForRole(%q, %v) = %v, want %v", tc.role, tc.caps, got, tc.allow)
			}
		})
	}
}
