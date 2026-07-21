# Tasks: Attestation & Audit Trail

> Prerequisite: `cross-harness-review` (coded_by/reviewed_by metadata trong state).

- [ ] 1.1 `pkg/attest/`: statement + PAE/DSSE + Ed25519 sign/verify + tests (tamper → fail) (REQ-002)
- [ ] 1.2 Keyset: load-or-generate + rotation (active/retired) + `GET /attestations/keys` JWKS (REQ-006)
- [ ] 1.3 Lưu `prompt_hash` tại code steps nếu chưa có trong state
- [ ] 1.4 Migration `attestations` (kèm cột `key_id`) + repository
- [ ] 1.5 PR step: build/insert/post summary comment (link + bảng commit→key_id, <2k chars), fail-soft (REQ-001, REQ-003)
- [ ] 1.6 `GET /attestations/{commit}` verify theo key_id của record, test rotation: record cũ vẫn pass (REQ-004)
- [ ] 1.7 UI Audit panel + verify badge (REQ-005)
- [ ] 1.8 E2E: task → PR → verify pass; sửa payload → fail
- [ ] 1.9 Update specs.md status
