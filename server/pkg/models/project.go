package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Valid execution_engine values for projects/tasks.
const (
	ExecutionEngineAPINative = "api_native"
	ExecutionEngineCLI       = "cli"
)

// ValidExecutionEngines lists the accepted execution_engine values.
var ValidExecutionEngines = map[string]bool{
	ExecutionEngineAPINative: true,
	ExecutionEngineCLI:       true,
}

// Valid review_harness_policy values for projects.
const (
	ReviewHarnessSame              = "same"
	ReviewHarnessDifferentModel    = "different_model"
	ReviewHarnessDifferentProvider = "different_provider"
)

// ValidReviewHarnessPolicies lists the accepted review_harness_policy values.
var ValidReviewHarnessPolicies = map[string]bool{
	ReviewHarnessSame:              true,
	ReviewHarnessDifferentModel:    true,
	ReviewHarnessDifferentProvider: true,
}

// ValidateReviewHarnessPolicy returns an error if policy is set but not recognized.
func ValidateReviewHarnessPolicy(policy string) error {
	if policy == "" {
		return nil
	}
	if !ValidReviewHarnessPolicies[policy] {
		return fmt.Errorf("invalid review_harness_policy %q: must be one of same, different_model, different_provider", policy)
	}
	return nil
}

// CLIEngineConfig configures the generic subprocess-CLI execution engine.
type CLIEngineConfig struct {
	Command          string            `json:"command"`                      // e.g. "claude"
	Args             []string          `json:"args"`                         // e.g. ["-p", "--dangerously-skip-permissions", "{prompt_file}"]
	CredentialID     string            `json:"credential_id,omitempty"`      // Link to ProviderCredential for CLI auth state
	Env              map[string]string `json:"env,omitempty"`                // masked as "***" in GET responses
	TimeoutMinutes   int               `json:"timeout_minutes"`              // default 30, max 120
	AuthCheckCommand string            `json:"auth_check_command,omitempty"` // e.g. "claude auth status"
	AllowNoop        bool              `json:"allow_noop,omitempty"`         // allow a run with zero file changes to succeed (read-only/no-op modes)
	// UnderlyingProvider optionally declares which API provider the CLI itself
	// runs on top of (e.g. "claude" CLI -> "anthropic"), so cross-harness review
	// can pick a genuinely different provider instead of assuming the CLI is
	// automatically "different" from every API provider (REQ-001b).
	UnderlyingProvider string `json:"underlying_provider,omitempty"`
	// ProfileRef is the CLIProfiles/ExecutionProviderConfig ref that produced
	// this config ("claude_code"/"openai_codex"/"antigravity"/"custom", or ""
	// for the legacy fallback path). Not user-settable — set by the Execution
	// Router when building this config, used to select the right CLIQuotaRules
	// entry for quota/rate-limit detection (REQ-006).
	ProfileRef string `json:"-"`
}

// ExecutionProviderConfig is one candidate in Project.ExecutionProviders — an
// API or CLI provider the Execution Router may pick for a Task, in priority
// order. See docs/openspecs/cli-execution-provider-routing.
type ExecutionProviderConfig struct {
	Type         string           `json:"type"`                    // "api" | "cli"
	Ref          string           `json:"ref"`                     // api: provider id (openai/anthropic/gemini/...); cli: "claude_code"/"openai_codex"/"antigravity"/"custom"
	CredentialID string           `json:"credential_id,omitempty"` // optional pin; empty = auto-pick first available
	Priority     int              `json:"priority"`
	Enabled      bool             `json:"enabled"`
	CLIConfig    *CLIEngineConfig `json:"cli_config,omitempty"` // only when ref=="custom"
}

var validExecutionProviderTypes = map[string]bool{"api": true, "cli": true}
var validCLIProviderRefs = map[string]bool{"claude_code": true, "openai_codex": true, "antigravity": true, "custom": true}

// ValidateExecutionProviders parses and validates a raw execution_providers
// jsonb payload. Returns (nil, nil) for an empty/absent payload — callers
// must treat that as "use legacy ExecutionEngine/CLIEngineConfig fallback"
// (REQ-003), not as an error.
func ValidateExecutionProviders(raw json.RawMessage) ([]ExecutionProviderConfig, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var list []ExecutionProviderConfig
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("execution_providers: invalid JSON: %w", err)
	}
	for i, p := range list {
		if !validExecutionProviderTypes[p.Type] {
			return nil, fmt.Errorf("execution_providers[%d].type must be \"api\" or \"cli\", got %q", i, p.Type)
		}
		if p.Type == "cli" {
			if !validCLIProviderRefs[p.Ref] {
				return nil, fmt.Errorf("execution_providers[%d].ref %q is not a known CLI profile", i, p.Ref)
			}
			// Config requirements for ref="custom" only matter once the row is
			// actually enabled; a disabled placeholder row (as the UI always
			// includes for every known profile) shouldn't block saving the rest
			// of the list.
			if p.Enabled && p.Ref == "custom" && (p.CLIConfig == nil || strings.TrimSpace(p.CLIConfig.Command) == "") {
				return nil, fmt.Errorf("execution_providers[%d]: ref=\"custom\" requires cli_config.command", i)
			}
			if p.Enabled && p.Ref == "custom" && strings.TrimSpace(p.CredentialID) == "" {
				return nil, fmt.Errorf("execution_providers[%d]: ref=\"custom\" requires credential_id", i)
			}
		}
		if p.Type == "api" && !IsAllowedProvider(p.Ref) {
			return nil, fmt.Errorf("execution_providers[%d].ref %q is not a known API provider", i, p.Ref)
		}
	}
	return list, nil
}

// MaskedEnv returns a copy of the config with env values redacted, preserving keys.
func (c CLIEngineConfig) MaskedEnv() CLIEngineConfig {
	if len(c.Env) == 0 {
		return c
	}
	masked := make(map[string]string, len(c.Env))
	for k := range c.Env {
		masked[k] = "***"
	}
	c.Env = masked
	return c
}

// Project groups repositories, agents, and rules under an organization.
type Project struct {
	ID                  string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID               string          `json:"org_id" gorm:"type:uuid;not null"`
	Name                string          `json:"name" gorm:"not null"`
	Description         string          `json:"description" gorm:"default:''"`
	DefaultModelLevel   string          `json:"default_model_level" gorm:"column:default_model_level;default:'balanced';not null"`
	DefaultAutonomy     string          `json:"default_autonomy" gorm:"column:default_autonomy;default:'supervised';not null"`
	AutoReviewPolicy    string          `json:"auto_review_policy" gorm:"column:auto_review_policy;default:'complexity_based';not null"`
	MaxRetries          int             `json:"max_retries" gorm:"column:max_retries;default:3;not null"`
	MaxReviewFixCycles  int             `json:"max_review_fix_cycles" gorm:"column:max_review_fix_cycles;default:3;not null"`
	DefaultBranch       string          `json:"default_branch" gorm:"column:default_branch;default:'main';not null"`
	ExecutionEngine     string          `json:"execution_engine" gorm:"column:execution_engine;default:'api_native';not null"`
	CLIEngineConfig     json.RawMessage `json:"cli_engine_config" gorm:"column:cli_engine_config;type:jsonb;default:'{}'"`
	ExecutionProviders  json.RawMessage `json:"execution_providers,omitempty" gorm:"column:execution_providers;type:jsonb;default:'[]'"`
	ReviewHarnessPolicy string          `json:"review_harness_policy" gorm:"column:review_harness_policy;default:'different_model';not null"`
	SmartRouting        bool            `json:"smart_routing" gorm:"column:smart_routing;default:true;not null"`
	PipelineConfig      json.RawMessage `json:"pipeline_config,omitempty" gorm:"column:pipeline_config;type:jsonb"`
	RepositoriesCount   int             `json:"repositories_count,omitempty" gorm:"->"`
	AgentsCount         int             `json:"agents_count,omitempty" gorm:"->"`
	TasksDoneCount      int             `json:"tasks_done_count,omitempty" gorm:"->"`
	TasksTotalCount     int             `json:"tasks_total_count,omitempty" gorm:"->"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// CreateProjectInput is the payload to create a project.
type CreateProjectInput struct {
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	DefaultModelLevel   *string          `json:"default_model_level,omitempty"`
	DefaultAutonomy     *string          `json:"default_autonomy,omitempty"`
	AutoReviewPolicy    *string          `json:"auto_review_policy,omitempty"`
	MaxRetries          *int             `json:"max_retries,omitempty"`
	MaxReviewFixCycles  *int             `json:"max_review_fix_cycles,omitempty"`
	DefaultBranch       *string          `json:"default_branch,omitempty"`
	ExecutionEngine     *string          `json:"execution_engine,omitempty"`
	CLIEngineConfig     *CLIEngineConfig `json:"cli_engine_config,omitempty"`
	ExecutionProviders  json.RawMessage  `json:"execution_providers,omitempty"`
	ReviewHarnessPolicy *string          `json:"review_harness_policy,omitempty"`
	SmartRouting        *bool            `json:"smart_routing,omitempty"`
	PipelineConfig      json.RawMessage  `json:"pipeline_config,omitempty"`
}

// UpdateProjectInput is the payload to partially update a project.
type UpdateProjectInput struct {
	Name                *string          `json:"name,omitempty"`
	Description         *string          `json:"description,omitempty"`
	DefaultModelLevel   *string          `json:"default_model_level,omitempty"`
	DefaultAutonomy     *string          `json:"default_autonomy,omitempty"`
	AutoReviewPolicy    *string          `json:"auto_review_policy,omitempty"`
	MaxRetries          *int             `json:"max_retries,omitempty"`
	MaxReviewFixCycles  *int             `json:"max_review_fix_cycles,omitempty"`
	DefaultBranch       *string          `json:"default_branch,omitempty"`
	ExecutionEngine     *string          `json:"execution_engine,omitempty"`
	CLIEngineConfig     *CLIEngineConfig `json:"cli_engine_config,omitempty"`
	ExecutionProviders  json.RawMessage  `json:"execution_providers,omitempty"`
	ReviewHarnessPolicy *string          `json:"review_harness_policy,omitempty"`
	SmartRouting        *bool            `json:"smart_routing,omitempty"`
	PipelineConfig      json.RawMessage  `json:"pipeline_config,omitempty"`
}

// ValidateExecutionEngine returns an error if engine is set but not a recognized value.
func ValidateExecutionEngine(engine string) error {
	if engine == "" {
		return nil
	}
	if !ValidExecutionEngines[engine] {
		return fmt.Errorf("invalid execution_engine %q: must be one of api_native, cli", engine)
	}
	return nil
}

// ValidateCLIEngineConfig checks that a CLI engine config is usable when engine=cli.
func ValidateCLIEngineConfig(engine string, cfg *CLIEngineConfig) error {
	if engine != ExecutionEngineCLI {
		return nil
	}
	if cfg == nil || strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("cli_engine_config.command is required when execution_engine is \"cli\"")
	}
	return nil
}
