package models

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// Task statuses — full lifecycle.
const (
	TaskStatusTodo           = "todo"
	TaskStatusContextLoading = "context_loading"
	TaskStatusAnalyzing      = "analyzing"
	TaskStatusSpecReview     = "spec_review"
	TaskStatusCoding         = "coding"
	TaskStatusReviewing      = "reviewing"
	TaskStatusFixing         = "fixing"
	TaskStatusTesting        = "testing"
	TaskStatusPrReady        = "pr_ready"
	TaskStatusHumanReview    = "human_review"
	TaskStatusMerged         = "merged"
	TaskStatusFailed         = "failed"
)

// Task complexity levels.
const (
	TaskComplexityEasy   = "easy"
	TaskComplexityMedium = "medium"
	TaskComplexityHard   = "hard"
)

// ValidTaskTransitions defines allowed status transitions.
// Note: TaskStatusContextLoading allows jumping directly to testing/review/PR for read-only or analysis tasks.
var ValidTaskTransitions = map[string][]string{
	TaskStatusTodo:           {TaskStatusContextLoading, TaskStatusAnalyzing, TaskStatusCoding},
	TaskStatusContextLoading: {TaskStatusAnalyzing, TaskStatusSpecReview, TaskStatusCoding, TaskStatusReviewing, TaskStatusTesting, TaskStatusPrReady, TaskStatusFailed},
	TaskStatusAnalyzing:      {TaskStatusSpecReview, TaskStatusCoding, TaskStatusReviewing, TaskStatusFixing, TaskStatusTesting, TaskStatusHumanReview, TaskStatusPrReady, TaskStatusMerged, TaskStatusFailed},
	TaskStatusSpecReview:     {TaskStatusCoding, TaskStatusTodo, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusCoding:         {TaskStatusReviewing, TaskStatusTesting, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusReviewing:      {TaskStatusFixing, TaskStatusTesting, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusFixing:         {TaskStatusReviewing, TaskStatusTesting, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusTesting:        {TaskStatusPrReady, TaskStatusFixing, TaskStatusFailed, TaskStatusMerged, TaskStatusReviewing, TaskStatusAnalyzing},
	TaskStatusPrReady:        {TaskStatusHumanReview, TaskStatusMerged, TaskStatusFailed, TaskStatusFixing, TaskStatusAnalyzing},
	TaskStatusHumanReview:    {TaskStatusMerged, TaskStatusFixing, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusMerged:         {},
	TaskStatusFailed:         {TaskStatusTodo, TaskStatusContextLoading, TaskStatusAnalyzing, TaskStatusSpecReview, TaskStatusCoding, TaskStatusReviewing, TaskStatusFixing, TaskStatusTesting, TaskStatusPrReady, TaskStatusHumanReview},
}

const (
	TaskSpecStatusNone                  = "none"
	TaskSpecStatusDraft                 = "draft"
	TaskSpecStatusPendingReview         = "pending_review"
	TaskSpecStatusChangesRequested      = "changes_requested"
	TaskSpecStatusClarificationRequired = "clarification_required"
	TaskSpecStatusApproved              = "approved"
	TaskSpecStatusAutoApproved          = "auto_approved"
	TaskSpecStatusReadyWithWarnings     = "ready_with_warnings"
)

// Task represents a unit of work for an agent.
type Task struct {
	ID              string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID       string          `json:"project_id" gorm:"type:uuid;not null"`
	AgentID         *string         `json:"agent_id,omitempty" gorm:"type:uuid"`
	ParentTaskID    *string         `json:"parent_task_id,omitempty" gorm:"type:uuid"`
	RepositoryID    *string         `json:"repository_id,omitempty" gorm:"type:uuid"`
	Title           string          `json:"title" gorm:"not null"`
	Description     string          `json:"description" gorm:"default:''"`
	Status          string          `json:"status" gorm:"default:'todo'"`
	Complexity      string          `json:"complexity" gorm:"default:'easy'"`
	Priority        int             `json:"priority" gorm:"default:0"`
	Labels          pq.StringArray  `json:"labels" gorm:"type:text[];default:'{}'"`
	Analysis        json.RawMessage `json:"analysis" gorm:"type:jsonb;default:'{}'"`
	SpecStatus      string          `json:"spec_status" gorm:"default:'none'"`
	Clarifications  json.RawMessage `json:"clarifications,omitempty" gorm:"type:jsonb;default:'[]'"`
	PRURLs          pq.StringArray  `json:"pr_urls" gorm:"type:text[]"`
	PRMetadata      json.RawMessage `json:"pr_metadata" gorm:"type:jsonb;default:'[]'"`
	ExecutionEngine *string         `json:"execution_engine,omitempty" gorm:"column:execution_engine"` // nil = inherit from project
	SubTasks        []Task          `json:"subtasks,omitempty" gorm:"foreignKey:ParentTaskID"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// CreateTaskInput is the payload to create a task.
type CreateTaskInput struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Complexity      string   `json:"complexity"`
	Priority        int      `json:"priority"`
	Labels          []string `json:"labels"`
	ParentTaskID    *string  `json:"parent_task_id,omitempty"`
	AgentID         *string  `json:"agent_id,omitempty"`
	RepositoryID    *string  `json:"repository_id,omitempty"`
	ExecutionEngine *string  `json:"execution_engine,omitempty"`
}

// UpdateTaskInput is the payload to partially update a task.
type UpdateTaskInput struct {
	Title           *string         `json:"title,omitempty"`
	Description     *string         `json:"description,omitempty"`
	Status          *string         `json:"status,omitempty"`
	Complexity      *string         `json:"complexity,omitempty"`
	Priority        *int            `json:"priority,omitempty"`
	Labels          []string        `json:"labels,omitempty"`
	AgentID         *string         `json:"agent_id,omitempty"`
	RepositoryID    *string         `json:"repository_id,omitempty"`
	Analysis        json.RawMessage `json:"analysis,omitempty"`
	SpecStatus      *string         `json:"spec_status,omitempty"`
	Clarifications  json.RawMessage `json:"clarifications,omitempty"`
	PRURLs          *pq.StringArray `json:"pr_urls,omitempty"`
	PRMetadata      json.RawMessage `json:"pr_metadata,omitempty"`
	ParentTaskID    *string         `json:"parent_task_id,omitempty"`
	ExecutionEngine *string         `json:"execution_engine,omitempty"`
}

type ComplexityDetails struct {
	Architecture   string `json:"architecture"`
	DataMigration  bool   `json:"data_migration"`
	BreakingChange bool   `json:"breaking_change"`
}

type RiskDetail struct {
	Risk        string `json:"risk"`
	Probability string `json:"probability"`
	Severity    string `json:"severity"`
	Owner       string `json:"owner"`
	Mitigation  string `json:"mitigation"`
}

type AffectedFile struct {
	Repo       string  `json:"repo"`
	File       string  `json:"file"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type TaskDAG struct {
	ID         string             `json:"id"`
	DependsOn  []string           `json:"depends_on"`
	Complexity *ComplexityDetails `json:"complexity,omitempty"`
}

type ExecutionPhase struct {
	Phase string   `json:"phase"`
	Tasks []string `json:"tasks"`
}

type ExecutionProfile struct {
	Agent  string   `json:"agent"`
	Skills []string `json:"skills"`
}

type ExecutionConstraints struct {
	Parallelizable  bool    `json:"parallelizable"`
	MaxFiles        int     `json:"max_files"`
	EstimatedTokens int     `json:"estimated_tokens"`
	MaxRisk         string  `json:"max_risk"`
	RiskMultiplier  float64 `json:"risk_multiplier,omitempty"`
}

type ExecutionUnit struct {
	ID               string               `json:"id"`
	Objective        string               `json:"objective"`
	Tasks            []string             `json:"tasks"`
	ExecutionProfile ExecutionProfile     `json:"execution_profile"`
	Constraints      ExecutionConstraints `json:"constraints"`
	Dependencies     []string             `json:"dependencies,omitempty"`
	TargetFiles      []string             `json:"target_files,omitempty"`
}

type ExecutionBoundary struct {
	Module       string   `json:"module"`
	Root         string   `json:"root"`
	Capabilities []string `json:"capabilities"`
	RepoName     string   `json:"repo_name,omitempty"`
	RepositoryID string   `json:"repository_id,omitempty"`
}

type ExpandedBoundary struct {
	File       string `json:"file"`
	Reason     string `json:"reason"`
	Capability string `json:"capability"`
	Risk       string `json:"risk"` // LOW, MEDIUM, HIGH, CRITICAL
}

// ReviewFinding is the typed contract crossing the review→fix seam.
// File is repository-relative by definition; Repo carries repository
// identity separately (never as a path prefix). This is the first
// applied slice of the execution-semantics-2026 typed-contract model.
type ReviewFinding struct {
	Repo           string `json:"repo,omitempty"`
	File           string `json:"file"` // repository-relative
	Line           int    `json:"line,omitempty"`
	Severity       string `json:"severity"` // CRITICAL|HIGH|MEDIUM|LOW
	Recommendation string `json:"recommendation"`
	// RequiresFix preserves the legacy boolean actionability signal some reviewer
	// outputs use instead of (or alongside) Severity — independent OR condition,
	// not derived from Severity.
	RequiresFix bool `json:"requires_fix,omitempty"`
}

type TaskAnalysis struct {
	Complexity             string              `json:"complexity"`
	PrimaryCategory        string              `json:"primary_category,omitempty"`
	SpecHash               string              `json:"spec_hash,omitempty"`
	Scope                  string              `json:"scope"`
	AffectedFiles          []AffectedFile      `json:"affected_files"`
	Risks                  []string            `json:"risks"`
	ExecutionPhases        []ExecutionPhase    `json:"execution_phases,omitempty"`
	ExecutionUnits         []ExecutionUnit     `json:"execution_units,omitempty"`
	ExecutionIRs           []ExecutionIR       `json:"execution_irs,omitempty"`
	ExecutionIRTargets     map[string][]string `json:"execution_ir_targets,omitempty"`
	ExecutionBoundaries    []ExecutionBoundary `json:"execution_boundaries,omitempty"`
	ExpandedBoundaries     []ExpandedBoundary  `json:"expanded_boundaries,omitempty"`
	AcceptanceCriteria     []map[string]any    `json:"acceptance_criteria,omitempty"`
	ClarificationQuestions []string            `json:"clarification_questions,omitempty"`
	TaskRules              []string            `json:"task_rules,omitempty"`
	RequiredSkills         []string            `json:"required_skills,omitempty"`
	RiskDomains            []string            `json:"risk_domains,omitempty"`
	ProposalMD             string              `json:"proposal_md,omitempty"`
	SpecsMD                string              `json:"specs_md,omitempty"`
	DesignMD               string              `json:"design_md,omitempty"`
	TasksMD                string              `json:"tasks_md,omitempty"`
	Tasks                  []TaskDAG           `json:"tasks,omitempty"`
	ComplexityDetails      *ComplexityDetails  `json:"complexity_details,omitempty"`
	RisksDetails           []RiskDetail        `json:"risks_details,omitempty"`
	RequiredSkillsMap      map[string][]string `json:"required_skills_map,omitempty"`
	RetryCount             int                 `json:"retry_count,omitempty"`
}

// FrozenContext holds the immutable execution contract for a workflow run.
type FrozenContext struct {
	SpecHash            string              `json:"spec_hash"`
	ProposalMD          string              `json:"proposal_md"`
	SpecsMD             string              `json:"specs_md"`
	DesignMD            string              `json:"design_md"`
	TasksMD             string              `json:"tasks_md"`
	ExecutionUnits      []ExecutionUnit     `json:"execution_units"`
	ExecutionIRs        []ExecutionIR       `json:"execution_irs"`
	ExecutionIRTargets  map[string][]string `json:"execution_ir_targets"`
	ExecutionBoundaries []ExecutionBoundary `json:"execution_boundaries"`
	AffectedFiles       []AffectedFile      `json:"affected_files"`
	AcceptanceCriteria  []map[string]any    `json:"acceptance_criteria"`
	ExecutionPhases     []ExecutionPhase    `json:"execution_phases"`
	Risks               []string            `json:"risks"`
	RiskDomains         []string            `json:"risk_domains"`
}

type TaskSpecProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// TaskSpec is the 4-file OpenSpec bundle authored by the CLI spec-first
// flow's cli_spec step, read live from the task's worktree.
type TaskSpec struct {
	Proposal string           `json:"proposal"`
	Specs    string           `json:"specs"`
	Design   string           `json:"design"`
	Tasks    string           `json:"tasks"`
	Progress TaskSpecProgress `json:"progress"`
}

type ClarifyTaskInput struct {
	Context string `json:"context"`
}

type ClarificationRound struct {
	Round     int       `json:"round"`
	Timestamp time.Time `json:"timestamp"`
	Questions []string  `json:"questions"`
	Response  string    `json:"response"`
}
