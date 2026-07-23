// Package attest implements in-toto/DSSE-style signed attestations for
// commits produced by Auto Code OS (P4.3): a Statement describing who/what
// coded and reviewed a commit under which policy, wrapped in a DSSE
// envelope and signed with a per-deployment Ed25519 key.
package attest

import "time"

// StatementType and PredicateType identify this attestation's shape per the
// in-toto v1 attestation spec (design.md).
const (
	StatementType = "https://in-toto.io/Statement/v1"
	PredicateType = "https://autocode.os/attestation/v1"
)

// Actor identifies the engine/provider/model that performed a coding or
// review step.
type Actor struct {
	Engine   string `json:"engine,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// PolicySnapshot captures the governance/policy state in effect when the
// commit was produced, for audit purposes.
type PolicySnapshot struct {
	Autonomy      string `json:"autonomy,omitempty"`
	ReviewHarness string `json:"review_harness,omitempty"`
	FixCyclesUsed int    `json:"fix_cycles_used"`
}

// Predicate is the Auto Code OS-specific attestation payload.
type Predicate struct {
	CodedBy    Actor          `json:"coded_by"`
	ReviewedBy *Actor         `json:"reviewed_by,omitempty"`
	PromptHash string         `json:"prompt_hash,omitempty"`
	Policy     PolicySnapshot `json:"policy"`
	TaskID     string         `json:"task_id"`
	JobID      string         `json:"job_id"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Subject identifies the commit this statement is about.
type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// Statement is the in-toto v1 Statement wrapping Predicate.
type Statement struct {
	Type          string    `json:"_type"`
	Subject       []Subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
	Predicate     Predicate `json:"predicate"`
}

// BuildStatement constructs a Statement for one commit.
func BuildStatement(repoName, commitHash string, predicate Predicate) *Statement {
	return &Statement{
		Type: StatementType,
		Subject: []Subject{{
			Name:   repoName + "@" + commitHash,
			Digest: map[string]string{"gitCommit": commitHash},
		}},
		PredicateType: PredicateType,
		Predicate:     predicate,
	}
}
