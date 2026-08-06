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
	// TaskStatusBlocked is reachable only from a running decomposed parent
	// task whose current child (by SequenceIndex) failed — it is never a
	// valid destination for a non-decomposed task (docs/openspecs/
	// task-subtask-decomposition/specs.md Invariants). Unlike TaskStatusFailed,
	// it does not imply "this work did not happen": completed sibling
	// children's progress remains intact and the parent is resumable.
	TaskStatusBlocked = "blocked"
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
	TaskStatusCoding:         {TaskStatusReviewing, TaskStatusTesting, TaskStatusHumanReview, TaskStatusFailed, TaskStatusAnalyzing, TaskStatusBlocked, TaskStatusMerged},
	TaskStatusReviewing:      {TaskStatusFixing, TaskStatusTesting, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusFixing:         {TaskStatusReviewing, TaskStatusTesting, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusTesting:        {TaskStatusPrReady, TaskStatusFixing, TaskStatusFailed, TaskStatusMerged, TaskStatusReviewing, TaskStatusAnalyzing},
	TaskStatusPrReady:        {TaskStatusHumanReview, TaskStatusMerged, TaskStatusFailed, TaskStatusFixing, TaskStatusAnalyzing},
	TaskStatusHumanReview:    {TaskStatusPrReady, TaskStatusMerged, TaskStatusFixing, TaskStatusFailed, TaskStatusAnalyzing},
	TaskStatusMerged:         {},
	TaskStatusFailed:         {TaskStatusTodo, TaskStatusContextLoading, TaskStatusAnalyzing, TaskStatusSpecReview, TaskStatusCoding, TaskStatusReviewing, TaskStatusFixing, TaskStatusTesting, TaskStatusPrReady, TaskStatusHumanReview},
	// TaskStatusBlocked (decomposed-parent-only, see the constant's doc comment):
	// retrying resumes to TaskStatusCoding (redispatches the failed child in
	// place), or the parent proceeds forward to TaskStatusMerged once every
	// child eventually succeeds and Reduce completes; TaskStatusFailed remains
	// reachable for an operator who abandons the decomposed run entirely.
	TaskStatusBlocked: {TaskStatusCoding, TaskStatusMerged, TaskStatusFailed},
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
	// PausedStep records which workflow step raised the clarification pause
	// (docs/openspecs/cli-execution-reliability, REQ-006) — empty for the
	// legacy API-native flow, where clarification always originates at, and
	// resumes to, the "analyze" step. Set only by the CLI spec-first steps
	// (cli_analyze/cli_spec/cli_implement).
	PausedStep      string          `json:"paused_step,omitempty" gorm:"default:''"`
	PRURLs          pq.StringArray  `json:"pr_urls" gorm:"type:text[]"`
	PRMetadata      json.RawMessage `json:"pr_metadata" gorm:"type:jsonb;default:'[]'"`
	ExecutionEngine *string         `json:"execution_engine,omitempty" gorm:"column:execution_engine"` // nil = inherit from project
	SubTasks        []Task          `json:"subtasks,omitempty" gorm:"foreignKey:ParentTaskID"`
	// SequenceIndex is this task's 0-based dispatch order among its siblings
	// when it is a decomposed child (nil for a non-decomposed task or a
	// decomposition parent). Children execute strictly in this order in v1
	// (docs/openspecs/task-subtask-decomposition).
	SequenceIndex *int `json:"sequence_index,omitempty"`
	// DecompositionMode is "manual" | "auto" | "disabled", resolved at
	// analyze time (task override, else project default, else "manual").
	// Nil means "not yet resolved for this task" and behaves like the
	// pre-existing single-task path (no auto-split).
	DecompositionMode *string `json:"decomposition_mode,omitempty"`
	// ComplexityScore is the analyze-time split-risk breakdown (tokens,
	// affected files, dependency depth, deliverable count, total), stored
	// regardless of whether a split was proposed (telemetry/tuning value).
	ComplexityScore json.RawMessage `json:"complexity_score,omitempty" gorm:"type:jsonb"`
	// DependsOn holds sibling child-task IDs this task's Contract declared a
	// dependency on. Captured from Phase 1 but not used for scheduling in
	// v1 — dispatch is strictly SequenceIndex order (see proposal.md Non-goals).
	DependsOn pq.StringArray `json:"depends_on,omitempty" gorm:"type:text[]"`
	// BlockedChildID is the child task ID whose failure put this (parent)
	// task into TaskStatusBlocked. Nil unless Status == TaskStatusBlocked.
	BlockedChildID *string   `json:"blocked_child_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorkspaceOwnerID returns the task ID whose on-disk workspace and git
// branch a given task's execution should use. A decomposed child (v1 only
// supports a single level of parent/child, so ParentTaskID is always the
// root) shares its parent's workspace/branch lineage so children execute
// sequentially on the same worktree, each committing on top of the last,
// rather than each getting an isolated clone
// (docs/openspecs/task-subtask-decomposition/design.md, "Key Decisions").
// Every other identity concern (logging, checkpoints, TaskAttempt rows, PR
// association, telemetry) must keep using task.ID directly, not this value.
func WorkspaceOwnerID(task *Task) string {
	if task.ParentTaskID != nil && *task.ParentTaskID != "" {
		return *task.ParentTaskID
	}
	return task.ID
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
	PausedStep      *string         `json:"paused_step,omitempty"`
	PRURLs          *pq.StringArray `json:"pr_urls,omitempty"`
	PRMetadata      json.RawMessage `json:"pr_metadata,omitempty"`
	ParentTaskID    *string         `json:"parent_task_id,omitempty"`
	ExecutionEngine *string         `json:"execution_engine,omitempty"`
	SequenceIndex     *int             `json:"sequence_index,omitempty"`
	DecompositionMode *string          `json:"decomposition_mode,omitempty"`
	ComplexityScore   json.RawMessage  `json:"complexity_score,omitempty"`
	DependsOn         *pq.StringArray  `json:"depends_on,omitempty"`
	BlockedChildID    *string          `json:"blocked_child_id,omitempty"`
	ClearBlockedChild bool             `json:"-"`
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

// ComplexityScore is the analyze-time, multi-factor split-risk breakdown
// (docs/openspecs/task-subtask-decomposition). A single input-token
// threshold under/over-triggers depending on task shape, so risk is
// computed from four independent factors and summed into Total.
type ComplexityScore struct {
	Tokens           int `json:"tokens"`
	Files            int `json:"files"`
	DependencyDepth  int `json:"dependency_depth"`
	DeliverableCount int `json:"deliverable_count"`
	Total            int `json:"total"`
}

// ChildTaskContract is the Contract carried by a proposed ChildTaskSpec:
// what the child can assume as input, and what it is expected to produce.
// The Contract makes a wrong split boundary detectable immediately (a child
// that doesn't touch its OutputExpected files) instead of surfacing only as
// a confusing downstream failure.
type ChildTaskContract struct {
	InputPreviousSummary *ChildTaskSummary `json:"input_previous_summary,omitempty"`
	OutputExpected       []string          `json:"output_expected"`
}

// ChildTaskSpec is analyze's pre-approval output: one proposed child task in
// an ordered split. DependsOn holds indices into the same proposed list
// (resolved to real task IDs only once children are actually created).
type ChildTaskSpec struct {
	Title        string            `json:"title"`
	Instructions string            `json:"instructions"`
	Contract     ChildTaskContract `json:"contract"`
	DependsOn    []int             `json:"depends_on,omitempty"`
}

// ChildTaskSummary is a child task's post-execution structured outcome —
// the only thing the next sibling or the Reduce step ever consumes, never a
// child's raw CLI transcript (docs/openspecs/task-subtask-decomposition
// Rules: "The Reduce aggregation step never re-sends a child's full raw CLI
// transcript...").
type ChildTaskSummary struct {
	TaskID            string   `json:"task_id"`
	SequenceIndex     int      `json:"sequence_index"`
	ChangedFiles      []string `json:"changed_files"`
	TestsPassed       int      `json:"tests_passed"`
	TestsFailed       int      `json:"tests_failed"`
	CostUSD           float64  `json:"cost_usd"`
	DurationSeconds   int      `json:"duration_seconds"`
	OneLineOutcome    string   `json:"one_line_outcome"`
	ContractDeviation *string  `json:"contract_deviation,omitempty"`
}

// DecomposedTaskSummary is the parent's deterministic Reduce output: the
// aggregate of every child's ChildTaskSummary, plus decomposition metrics.
type DecomposedTaskSummary struct {
	ChangedFiles       []string           `json:"changed_files"`
	TestsPassed        int                `json:"tests_passed"`
	TestsFailed        int                `json:"tests_failed"`
	CostUSD            float64            `json:"cost_usd"`
	DurationSeconds    int                `json:"duration_seconds"`
	Children           []ChildTaskSummary `json:"children"`
	TokensBeforeSplit  int                `json:"tokens_before_split"`
	TokensAfterSplit   int                `json:"tokens_after_split"`
	DurationSingleEst  int                `json:"duration_single_estimate_seconds"`
	CostSavedUSD       float64            `json:"cost_saved_usd"`
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

// SpecViolation is a single spec-compliance finding from the structured 2-verdict
// review output (spec_compliance axis).
type SpecViolation struct {
	Requirement string `json:"requirement"`
	Observed    string `json:"observed"`
	Severity    string `json:"severity,omitempty"`
}

// QualityIssue is a single code-quality finding from the structured 2-verdict
// review output (code_quality axis).
type QualityIssue struct {
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ReviewVerdict is the structured 2-verdict review output: spec compliance is
// judged separately from code quality so the two axes can route differently
// (spec failures are more severe and can trigger escalation; quality failures
// go through the ordinary fix cycle).
type ReviewVerdict struct {
	SpecCompliance struct {
		Verdict    string          `json:"verdict"`
		Violations []SpecViolation `json:"violations,omitempty"`
	} `json:"spec_compliance"`
	CodeQuality struct {
		Verdict string         `json:"verdict"`
		Issues  []QualityIssue `json:"issues,omitempty"`
	} `json:"code_quality"`
	Summary string `json:"summary,omitempty"`
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
	IncludeSpecInMR        bool                `json:"include_spec_in_mr"`
	SpecFeedbackText       string              `json:"spec_feedback_text,omitempty"`
	Tasks                  []TaskDAG           `json:"tasks,omitempty"`
	ComplexityDetails      *ComplexityDetails  `json:"complexity_details,omitempty"`
	RisksDetails           []RiskDetail        `json:"risks_details,omitempty"`
	RequiredSkillsMap      map[string][]string `json:"required_skills_map,omitempty"`
	RetryCount             int                 `json:"retry_count,omitempty"`
	// ComplexityScore and ProposedSplit are the task-subtask-decomposition
	// feature's analyze-time output (docs/openspecs/task-subtask-decomposition).
	// ComplexityScore is always computed and recorded; ProposedSplit is only
	// populated when the score crosses the configured threshold and
	// decomposition_mode isn't "disabled".
	ComplexityScore *ComplexityScore `json:"complexity_score,omitempty"`
	ProposedSplit   []ChildTaskSpec  `json:"proposed_split,omitempty"`
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

type UpdateTaskSpecRequest struct {
	Proposal *string `json:"proposal,omitempty"`
	Specs    *string `json:"specs,omitempty"`
	Design   *string `json:"design,omitempty"`
	Tasks    *string `json:"tasks,omitempty"`
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
