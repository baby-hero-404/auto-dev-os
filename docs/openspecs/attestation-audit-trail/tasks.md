# Tasks: Attestation & Audit Trail

> Prerequisite: `cross-harness-review` (coded_by/reviewed_by metadata trong state).

- [x] 1.1 `pkg/attest/`: statement + PAE/DSSE + Ed25519 sign/verify + tests (tamper → fail) (REQ-002)
- [x] 1.2 Keyset: load-or-generate + rotation (active/retired) + `GET /attestations/keys` JWKS (REQ-006)
- [x] 1.3 Lưu `prompt_hash` tại code steps nếu chưa có trong state — Deviation: `prompt_hash` được tính tại PR step làm `sha256(repoDiffText)` (diff toàn bộ commit của repo), không phải capture tại thời điểm code step assemble prompt LLM thực sự dùng. Đủ để chống-tamper (payload đổi → hash đổi record), nhưng không phải "hash của prompt request thực tế". Xem implementation notes.
- [x] 1.4 Migration `attestations` (kèm cột `key_id`) + repository
- [x] 1.5 PR step: build/insert attestation, fail-soft (REQ-001) — Deviation: phần "post summary comment (link + bảng commit→key_id, <2k chars)" của REQ-003 CHƯA implement. `steps.GitOpsClient`/gitops client hiện tại chỉ có `CreatePullRequest`, không có primitive `AddComment`/`UpdatePR`. Thêm primitive này + wiring provider (GitHub API) là scope riêng cho iteration sau. Sign+persist attestation record đã chạy (fail-soft: lỗi ký/lưu chỉ log warning, không chặn PR — xem `pr.go` attestation block).
- [x] 1.6 `GET /attestations/{commit}` verify theo key_id của record, test rotation: record cũ vẫn pass (REQ-004)
- [ ] 1.7 UI Audit panel + verify badge (REQ-005) — Deferred: backend endpoints (`GET /attestations/{commit}`, `GET /tasks/{taskID}/attestations`, `GET /attestations/keys`) đã đủ để 1 UI panel consume trực tiếp; component React chưa viết, theo pattern deferral đã dùng cho UI item khác trong roadmap.
- [x] 1.8 E2E: task → PR → verify pass; sửa payload → fail — implemented as `TestAttestationService_SignThenVerify_RoundTrip` (sign → persist → verify pass → tamper decoded envelope payload byte → verify fail) và `TestAttestationService_RotateKey_OldRecordsStillVerify` (record ký bằng key cũ vẫn verify pass sau khi key đó chuyển `retired`), ở service layer thay vì full HTTP task→PR e2e (repo không có test harness spin up sandbox/gitops thật).
- [x] 1.9 Update specs.md status
