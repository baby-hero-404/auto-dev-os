# Implementation notes: attestation-audit-trail (P4.3)

## Design

- `pkg/attest/`: in-toto v1 `Statement` + `Predicate` (coded_by, reviewed_by, prompt_hash, policy_snapshot, task_id, job_id, timestamp), wrapped in a hand-written DSSE envelope (`payloadType`, base64 `payload`, `signatures[]`). PAE (Pre-Authentication Encoding) implemented from the DSSE spec directly — no external DSSE dependency, per design.md's explicit guidance to keep this minimal.
- Ed25519 signing via Go's `crypto/ed25519` stdlib. `key_id = sha256(pub)[:8]` hex.
- Keyset model: `attestation_keys` table, one `active` key signs new attestations, `retired` keys stay around only for verifying their own historical records. Every attestation stores the exact `key_id` that signed it; verification always looks that key up by ID rather than "whatever is active now" — this is what makes rotation safe (REQ-004/REQ-006).
- Private signing key is encrypted at rest reusing the existing `service.SecretCipher` (AES-GCM), the same mechanism already used for `SecretService`/credential pools — no new crypto subsystem introduced.
- Signing is wired into the PR step (`internal/orchestrator/steps/pr.go`) via an `AttestationSigner` interface owned by the `steps` package, implemented on the concrete service through `AttestationSignerAdapter` in `internal/service` — this avoids an import cycle (`steps` doesn't import `service` directly) while `cmd/api/main.go` wires concrete service → adapter → orchestrator `Option`.
- Signing failures are fail-soft by design: a signing/persistence error is only logged as a warning and never blocks PR creation (REQ-001's explicit requirement).

## Deviations from spec

- **REQ-003 (PR comment)**: not implemented. Sign+persist runs, but posting a summary comment to the PR requires a comment-posting primitive that doesn't exist yet in the gitops client (`steps.GitOpsClient` only has `CreatePullRequest`, no `AddComment`/`UpdatePR`). Adding that primitive plus GitHub API wiring is scoped as separate follow-up work.
- **REQ-005 (Audit panel UI)**: backend is complete (`GET /tasks/{taskID}/attestations`, `GET /attestations/{commit}`, `GET /attestations/keys` JWKS) but no React component was built, consistent with the UI-deferral pattern used elsewhere in this roadmap.
- **prompt_hash source**: computed in `pr.go` as `sha256(repoDiffText)` — a hash of the full commit diff for the repo — rather than being captured at the point the code step actually assembles its LLM prompt. This still satisfies the anti-tamper property (any change to the attested diff changes the hash), but it is not literally "hash of the prompt sent to the model." Revisit if a stronger prompt-provenance guarantee is needed later.
- **E2E test (task 1.8)**: implemented at the service layer (`internal/service/attestation_test.go`: `TestAttestationService_SignThenVerify_RoundTrip`, `TestAttestationService_RotateKey_OldRecordsStillVerify`) rather than a full HTTP task→PR→verify flow, since this repo has no test harness that spins up a real sandbox/gitops environment.

## Testing

- `pkg/attest/dsse_test.go`: unit-level sign/verify round-trip, tamper detection, wrong-key rejection, missing-key-id rejection.
- `internal/repository/attestation_test.go`: sqlmock+postgres-dialect CRUD tests for both repos (pattern reused from `internal/repository/project_test.go` — no sqlite driver is vendored in this repo).
- `internal/service/attestation_test.go`: service-level sign→verify round trip and tamper-detection using a pre-seeded known keypair (avoids brittle sqlmock argument-capture for the generated key), plus a rotation test proving a retired key still verifies its own historical record.
- `internal/orchestrator/steps/pr_step_test.go`: `TestPRStep_SignsAttestationForCreatedPR` (signer called with expected fields when a commit hash is available) and `TestPRStep_AttestationSignFailureDoesNotBlockPR` (fail-soft: PR step still returns `workflow.ErrWaitingApproval` even when the signer errors).

## Gotchas

- GORM's generated `LIMIT $N` argument shows up as an extra positional arg beyond the `WHERE` clause's own bind params; sqlmock's `WithArgs` needs `sqlmock.AnyArg()` appended for the limit slot or it logs (non-fatal) "arguments do not match" and returns a query error — matters when a test needs the query to actually succeed (e.g. this service-layer round-trip test), not just to observe *some* error.
