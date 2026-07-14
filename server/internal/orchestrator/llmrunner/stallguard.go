package llmrunner

import (
	"encoding/json"
	"fmt"
	"strings"
)

var stallGuardReadOnlyTools = map[string]bool{
	"list_files":           true,
	"read_file":            true,
	"run_lint":             true,
	"run_tests":            true,
	"run_build":            true,
	"grep_search":          true,
	"find_symbol":          true,
	"git_status":           true,
	"git_diff":             true,
	"file_exists":          true,
	"read_spec":            true,
	"read_affected_files":  true,
}

type successfulCall struct {
	turn int
}

type stallGuard struct {
	successfulCalls map[string]successfulCall
}

func newStallGuard() *stallGuard {
	return &stallGuard{
		successfulCalls: make(map[string]successfulCall),
	}
}

func normalizeJSON(jsonStr string) string {
	var val any
	if err := json.Unmarshal([]byte(jsonStr), &val); err != nil {
		return jsonStr
	}
	canonicalBytes, err := json.Marshal(val)
	if err != nil {
		return jsonStr
	}
	return string(canonicalBytes)
}

func (g *stallGuard) Check(name, argsJSON string) (string, bool) {
	if name == "search_replace" {
		var args map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			searchVal, hasSearch := args["search"]
			replaceVal, hasReplace := args["replace"]
			if hasSearch && hasReplace {
				search, _ := searchVal.(string)
				replace, _ := replaceVal.(string)
				if search == replace {
					return "Error: this search_replace is a no-op (search and replace are identical). The file already contains this content. Make a real change, or move on.", true
				}
			}
		}
	}

	if stallGuardReadOnlyTools[name] {
		norm := normalizeJSON(argsJSON)
		key := name + ":" + norm
		if prev, ok := g.successfulCalls[key]; ok {
			return fmt.Sprintf("You already ran %s with these exact arguments at turn %d and got a successful result. Re-running it returns no new information. Either write to a file within your execution boundary now, or explain in your final summary why you cannot.", name, prev.turn), true
		}
	}

	return "", false
}

func (g *stallGuard) RecordSuccess(name, argsJSON string, turn int) {
	if stallGuardReadOnlyTools[name] {
		norm := normalizeJSON(argsJSON)
		key := name + ":" + norm
		g.successfulCalls[key] = successfulCall{turn: turn}
	} else {
		// Invalidate workspace-state-dependent read-only tools on successful edit/write tool
		for key := range g.successfulCalls {
			if strings.HasPrefix(key, "run_tests:") || strings.HasPrefix(key, "run_build:") || strings.HasPrefix(key, "run_lint:") {
				delete(g.successfulCalls, key)
			}
		}
	}
}
