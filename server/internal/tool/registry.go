package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

// Registry manages the set of available tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool. It panics if a tool with the same name already exists.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Name()
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("duplicate tool registration: %s", name))
	}
	r.tools[name] = t
}

// Execute executes a tool by name with the given context and call parameters. Every call site
// (not just the diff-path boundary check in boundary_tool_executor.go) is subject to a
// role/capability check here, so a call to a tool outside call.AgentRole's DefaultRoleProfiles()
// capability set is rejected before the tool's Execute (and any filesystem mutation) runs.
func (r *Registry) Execute(ctx context.Context, name string, call Call) (Result, error) {
	r.mu.RLock()
	t, exists := r.tools[name]
	r.mu.RUnlock()
	if !exists {
		return Result{}, fmt.Errorf("unknown tool: %s", name)
	}
	if !AllowedForRole(call.AgentRole, t.Capabilities()) {
		var allowedToolNames []string
		r.mu.RLock()
		for tName, regTool := range r.tools {
			if AllowedForRole(call.AgentRole, regTool.Capabilities()) {
				allowedToolNames = append(allowedToolNames, tName)
			}
		}
		r.mu.RUnlock()
		sort.Strings(allowedToolNames)
		toolsList := strings.Join(allowedToolNames, ", ")

		return Result{
			Success: false,
			Message: fmt.Sprintf("Error: role %q is not authorized to use tool %q. This will not change during this step — do not call it again. Tools available to you: %s", call.AgentRole, name, toolsList),
		}, nil
	}
	return t.Execute(ctx, call)
}

// Definitions returns the native LLM tool definitions for all registered tools.
func (r *Registry) Definitions() []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Schema(),
		})
	}
	return defs
}

// ToolsForCapabilities returns filtered definitions matching any provided capability.
func (r *Registry) ToolsForCapabilities(caps []Capability) []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capMap := make(map[Capability]bool)
	for _, c := range caps {
		capMap[c] = true
	}

	var defs []llm.ToolDefinition
	for _, t := range r.tools {
		matched := false
		for _, tc := range t.Capabilities() {
			if capMap[tc] {
				matched = true
				break
			}
		}
		if matched {
			defs = append(defs, llm.ToolDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			})
		}
	}
	return defs
}
