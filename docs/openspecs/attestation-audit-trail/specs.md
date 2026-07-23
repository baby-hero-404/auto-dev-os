# Specs: Attestation & Audit Trail

## Added Requirements

### REQ-001: Attestation ghi tại PR
> ✅ Status: Done

**Scenario:**
- WHEN PR step tạo PR chứa các commit của task
- THEN mỗi commit có 1 attestation record với coded_by, reviewed_by, prompt_hash, policy_snapshot
- AND ghi fail → PR vẫn tạo, warning log + state flag (không chặn delivery)

Note: implemented trong `pr.go`'s per-repo loop — resolve HEAD commit hash, gọi `AttestationSigner.SignCommit`, lỗi (sign hoặc empty hash) chỉ log warning, không ảnh hưởng luồng tạo PR.

### REQ-002: DSSE envelope ký hợp lệ
> ✅ Status: Done

**Scenario:**
- WHEN attestation được serialize
- THEN envelope đúng DSSE spec (payloadType in-toto, payload base64, signatures[])
- AND verify bằng public key của deployment pass; sửa 1 byte payload → verify fail

Note: `pkg/attest/dsse.go` — PAE tự viết theo DSSE spec, Ed25519 qua `crypto/ed25519` stdlib. Covered bởi `dsse_test.go` (unit, round-trip + tamper) và `attestation_test.go` (service-level round-trip + tamper qua sqlmock).

### REQ-003: Đính vào PR (summary-only)
> ⚠️ Status: Partially done

**Scenario:**
- WHEN PR được tạo với M commits đã attest
- THEN PR có đúng 1 comment summary: `✅ M commits attested by Auto Code OS` + link tới Audit panel/API — KHÔNG chứa full envelope JSON
- AND comment luôn < 2,000 chars bất kể số commit (không bao giờ chạm giới hạn 65,536 của GitHub)

Note: Sign+persist per-commit đã chạy fail-soft (REQ-001). Phần post PR comment CHƯA implement — không có primitive comment-lên-PR trong gitops client hiện tại (`CreatePullRequest` only). Cần thêm `AddPRComment`-style method + provider wiring (GitHub API) trước khi làm phần này; scoped cho iteration sau, xem implementation notes.

### REQ-004: Verify endpoint đa key
> ✅ Status: Done

**Scenario:**
- WHEN GET `/attestations/{commit_hash}`
- THEN trả envelope + `{verified: true|false, key_id}`; commit không có attestation → 404
- AND verify dùng đúng key theo `key_id` lưu trong record (không phải chỉ key hiện tại)

**Scenario:**
- WHEN deployment đã rotate key sau khi record cũ được ký
- THEN record cũ vẫn verify pass bằng key cũ trong key set; record mới ký bằng key mới

Note: `AttestationHandler.GetByCommit` (404 qua `gorm.ErrRecordNotFound`), `AttestationService.VerifyByCommitHash` luôn load key theo `a.KeyID` recorded trên record. Test: `TestAttestationService_RotateKey_OldRecordsStillVerify`.

### REQ-005: Audit panel
> ⚠️ Status: Deferred (backend done, UI not built)

**Scenario:**
- WHEN task detail mở với task đã có PR
- THEN panel Audit hiển thị chain coded_by → reviewed_by → attested (verify badge) theo từng commit

Note: Backend hoàn chỉnh (`GET /tasks/{taskID}/attestations`, `GET /attestations/{commit}`, `GET /attestations/keys`). React component chưa viết — theo pattern deferral UI đã dùng ở các spec khác trong roadmap.

### REQ-006: Key management + rotation
> ✅ Status: Done

**Scenario:**
- WHEN server khởi động lần đầu không có signing key
- THEN sinh Ed25519 keypair với `key_id = sha256(pub)[:8]`, lưu private như secret hiện có

**Scenario:**
- WHEN admin rotate key (thêm key mới, key cũ chuyển trạng thái `retired` — vẫn verify được, không ký mới)
- THEN `GET /attestations/keys` trả JWKS-format list toàn bộ public keys (active + retired) kèm key_id
- AND mọi attestation record lưu `key_id` của key đã ký nó (cột riêng trong bảng)

## Modified Requirements
- Không có (PR step thêm hành vi, không đổi hành vi cũ).

## Removed Requirements
- Không có.
