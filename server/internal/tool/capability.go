package tool

import (
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

// RoleProfile defines the capabilities granted to an agent role.
type RoleProfile struct {
	Role         string
	Capabilities []Capability
}

// DefaultRoleProfiles returns the standard role -> capability mapping.
func DefaultRoleProfiles() map[string]RoleProfile {
	return map[string]RoleProfile{
		"planner": {
			Role:         "planner",
			Capabilities: []Capability{CapRead, CapSearch, CapContext, CapDocs},
		},
		"backend": {
			Role:         "backend",
			Capabilities: []Capability{CapRead, CapEdit, CapBuild, CapGit, CapSearch, CapContext},
		},
		"frontend": {
			Role:         "frontend",
			Capabilities: []Capability{CapRead, CapEdit, CapBuild, CapSearch, CapContext},
		},
		"reviewer": {
			Role:         "reviewer",
			Capabilities: []Capability{CapRead, CapSearch, CapGitDiff, CapContext},
		},
		"qa": {
			Role:         "qa",
			Capabilities: []Capability{CapRead, CapSearch, CapBuild, CapContext},
		},
		"security-auditor": {
			Role:         "security-auditor",
			Capabilities: []Capability{CapRead, CapSearch, CapDependency},
		},
	}
}

// CapabilityManager resolves role -> tool set.
type CapabilityManager struct {
	profiles map[string]RoleProfile
	registry *Registry
}

// NewCapabilityManager creates a CapabilityManager with the given registry and profiles.
func NewCapabilityManager(registry *Registry, profiles map[string]RoleProfile) *CapabilityManager {
	return &CapabilityManager{profiles: profiles, registry: registry}
}

// ToolsForRole returns the LLM tool definitions available to a role.
func (cm *CapabilityManager) ToolsForRole(role string) []llm.ToolDefinition {
	profile, ok := cm.profiles[strings.ToLower(role)]
	if !ok {
		// Fallback: read + search
		return cm.registry.ToolsForCapabilities([]Capability{CapRead, CapSearch})
	}
	return cm.registry.ToolsForCapabilities(profile.Capabilities)
}
