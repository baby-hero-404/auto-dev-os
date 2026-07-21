# Tasks: Review Verdict Split

- [ ] 1.1 `prompts/steps/review.md`: yêu cầu JSON schema 2-verdict + ví dụ
- [ ] 1.2 Parser trong `steps/review.go` (pattern analyze_parser) + fallback legacy + tests (format mới/cũ/rác)
- [ ] 1.3 Lưu violations vào step state cho cycle sau
- [ ] 1.4 Routing trong worker: spec-first, repeat-violation detection (token-set overlap ≥0.6) + tests
- [ ] 1.5 Escalation pause `awaiting_review_escalation` (reuse pause helper) + resume actions (continue/cancel)
- [ ] 1.6 `coding_instruction.go`: violations-first section trong fix instruction + tests
- [ ] 1.7 UI: 2 badges + expandable lists; legacy render (REQ-M01)
- [ ] 1.8 Integration test: spec fail 2 cycles → escalation
- [ ] 1.9 Update specs.md status
