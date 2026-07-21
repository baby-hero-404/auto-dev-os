# Proposal: Attestation & Audit Trail (P4.3)

## Why

Code do agent viết cần trả lời được: ai viết, model nào, prompt nào, ai review, theo policy nào — yêu cầu bắt buộc cho enterprise adoption. ai-sdlc dùng DSSE envelope ký số; Auto Code OS đã có mảnh nền từ `cross-harness-review` (coded_by/reviewed_by metadata). Set này hoàn thiện thành audit trail đầy đủ, có ký, gắn vào PR.

## What Changes

### Issue 1: Attestation record

- Bảng `attestations`: `task_id, job_id, commit_hash, coded_by (jsonb), reviewed_by (jsonb), prompt_hash (sha256 của system+instruction), policy_snapshot (jsonb — engine, harness policy, autonomy, cycles used), created_at`.
- Ghi tại PR step: mỗi commit trong PR có 1 record.

### Issue 2: DSSE-style envelope + ký

- Serialize attestation thành in-toto Statement JSON, bọc DSSE envelope, ký Ed25519 (key per-deployment, sinh lúc setup, lưu như secret hiện có).
- Envelope đính vào PR: comment block hoặc file `.attestations/<commit>.json` trong branch (chọn khi implement — comment không bẩn repo, file thì verify offline được; default: PR comment + lưu DB).

### Issue 3: Verify + UI

- CLI/endpoint verify: `GET /attestations/{commit}` trả envelope + kết quả verify chữ ký.
- Task detail: panel Audit hiển thị chain (coded → reviewed → attested) + badge verify.

## Capabilities

### New Capabilities
- Signed attestation per commit; verify endpoint; audit panel.

### Modified Capabilities
- PR step ghi attestation + đính envelope.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Migration + repo | bảng `attestations` |
| Crypto | pkg mới `pkg/attest/` (statement, dsse, sign/verify) |
| PR step | `steps/pr.go` |
| API/Web | verify endpoint + Audit panel |
