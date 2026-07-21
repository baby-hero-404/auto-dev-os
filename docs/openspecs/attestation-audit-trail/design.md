# Design: Attestation & Audit Trail

## Statement (in-toto v1)

```jsonc
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [{"name": "<repo>@<commit>", "digest": {"gitCommit": "<hash>"}}],
  "predicateType": "https://autocode.os/attestation/v1",
  "predicate": {
    "coded_by": {"engine": "cli|api_native", "provider": "...", "model": "..."},
    "reviewed_by": {...} | null,
    "prompt_hash": "sha256:...",
    "policy": {"autonomy": "...", "review_harness": "...", "fix_cycles_used": 2},
    "task_id": "...", "job_id": "...", "timestamp": "..."
  }
}
```

DSSE envelope chuẩn (`payloadType: application/vnd.in-toto+json`), ký Ed25519 — `crypto/ed25519` stdlib, PAE encoding tự viết (~30 dòng theo DSSE spec), không cần dependency ngoài.

## pkg/attest

```
pkg/attest/
├── statement.go   // build Statement từ step state
├── dsse.go        // PAE, Envelope, Sign, Verify(envelope, keyset)
└── keys.go        // keyset: load-or-generate; rotation (active + retired); JWKS export
```

**Key rotation model**: keyset = list `{key_id: sha256(pub)[:8], pub, priv?, status: active|retired}`. Chỉ 1 key `active` (ký mới); retired keys giữ để verify record cũ. Bảng `attestations` có cột `key_id` — verify endpoint lookup key theo key_id của record, KHÔNG dùng key hiện tại mặc định. `GET /attestations/keys` trả JWKS để bên ngoài verify offline.

`prompt_hash`: sha256 của system prompt + coding instruction đã assemble (lấy từ state nếu Wave 1-3 đã lưu; nếu chưa → thêm việc lưu hash tại code step — ghi vào tasks).

## PR integration

`steps/pr.go` sau khi có PR number: build statements từ `git log` các commit của branch + step state metadata → insert DB → post **1 comment summary duy nhất**: `✅ M commits attested by Auto Code OS · [View audit log](<ui-link>)` + bảng ngắn commit-hash → key_id. Full envelope JSON chỉ serve qua `GET /attestations/{commit}` — không bao giờ nhét vào comment (giới hạn 65,536 chars của GitHub + PR đọc được). Fail-soft (REQ-001).

## Trade-offs

- Ed25519 per-deployment keyset (có rotation), không PKI/keyless (sigstore): đủ cho "tamper-evident + truy vết nội bộ"; keyless là enhancement enterprise sau.
- PR comment summary-only thay vì full envelope hoặc commit file `.attestations/`: không bẩn repo user, không chạm comment size limit; DB + API là nguồn verify chính, comment là con trỏ.
- Không ký từng commit lúc tạo (checkpoint) mà ký lúc PR: ít record rác từ các checkpoint bị salvage/rollback.
