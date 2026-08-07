package models

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fixturePath points at the checked-in snapshot of TaskStatus values that the
// frontend's parity test (web/src/lib/status/__tests__/parity.test.ts) also
// reads. Keeping both sides pointed at one file is what turns "someone added
// a status on one side and forgot the other" into a hard CI failure instead
// of a hand-sync convention — see docs/openspecs/status-driven-agent-workspace/
// tasks.md, task 3.1a.
const fixturePath = "../../../docs/openspecs/status-driven-agent-workspace/task-statuses.generated.json"

// declaredTaskStatuses parses task.go's source (rather than hardcoding a
// second copy of the list here) so that adding a new TaskStatusXxx constant
// without regenerating the fixture actually fails this test, instead of the
// test silently staying in sync because it was hand-edited alongside it.
func declaredTaskStatuses(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "task.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse task.go: %v", err)
	}

	var statuses []string
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "TaskStatus") {
					continue
				}
				if i >= len(valueSpec.Values) {
					continue
				}
				lit, ok := valueSpec.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("failed to unquote %s literal %s: %v", name.Name, lit.Value, err)
				}
				statuses = append(statuses, value)
			}
		}
	}

	sort.Strings(statuses)
	return statuses
}

func readFixture(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", fixturePath, err)
	}

	var statuses []string
	if err := json.Unmarshal(data, &statuses); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", fixturePath, err)
	}

	sort.Strings(statuses)
	return statuses
}

// TestTaskStatusParity fails if server/pkg/models/task.go's TaskStatus*
// constants and task-statuses.generated.json drift apart. Regenerate the
// fixture (sorted JSON array of every TaskStatus string value) when this
// fails after a deliberate status addition/removal.
func TestTaskStatusParity(t *testing.T) {
	declared := declaredTaskStatuses(t)
	fixture := readFixture(t)

	if len(declared) != len(fixture) {
		t.Fatalf("declared TaskStatus constants (%v) don't match the fixture (%v) — regenerate %s", declared, fixture, fixturePath)
	}
	for i := range declared {
		if declared[i] != fixture[i] {
			t.Fatalf("declared TaskStatus constants (%v) don't match the fixture (%v) — regenerate %s", declared, fixture, fixturePath)
		}
	}
}
