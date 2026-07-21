# Proposal: RepoMap Mention Boost (P3.2)

## Why

`context/repomap/ranking.go` đã có PageRank với active-file boost 50x (trùng khớp độc lập với Aider — verified). Mảnh còn thiếu của công thức Aider: **mentioned_idents ×10** — identifier được nhắc trong task description phải kéo file/symbol liên quan lên hạng, vì đó là tín hiệu ý định trực tiếp nhất từ user. Hiện task nói "sửa `CreateGitCheckpoint`" nhưng repomap có thể vẫn xếp file chứa nó thấp nếu ít được tham chiếu.

## What Changes

### Issue 1: Identifier extraction từ task

- Extract candidate identifiers từ task title + description: token trong backticks, CamelCase/snake_case words, path-like strings (`a/b.go`).
- Lọc против danh sách definitions thực có trong repomap graph (chỉ boost cái tồn tại).

### Issue 2: Edge boost

- Trong ranking pass: edges trỏ tới definition của mentioned ident nhân trọng số ×10 (cùng cơ chế với active-file ×50 hiện có).
- File được mention trực tiếp theo path → treat như active file (×50).

## Capabilities

### New Capabilities
- Task-intent-aware ranking trong repomap.

### Modified Capabilities
- Ranking formula thêm 1 boost term; API `BuildRepoMap` nhận danh sách mentioned idents (hoặc raw task text).

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| RepoMap | `server/internal/context/repomap/ranking.go`, file mới `mentions.go` |
| Caller | nơi build repomap trong context_load truyền task text |
