# Specs: RepoMap Mention Boost

## Added Requirements

### REQ-001: Identifier extraction
> ❌ Status: Not Started

**Scenario:**
- WHEN task description chứa `` `CreateGitCheckpoint` ``, `policy_engine.go`, và từ thường "checkpoint"
- THEN extractor trả về `CreateGitCheckpoint` (backtick), `policy_engine.go` (path-like)
- AND từ thường không phải identifier trong graph KHÔNG được trả về (lọc theo definitions tồn tại)

### REQ-002: Symbol mention boost ×10
> ❌ Status: Not Started

**Scenario:**
- WHEN `CreateGitCheckpoint` được mention và tồn tại trong graph
- THEN edges tới definition đó nhân ×10, file chứa nó xếp hạng cao hơn baseline không-mention (assert bằng rank comparison test)

### REQ-003: Path mention = active file
> ❌ Status: Not Started

**Scenario:**
- WHEN task mention path khớp file trong repo
- THEN file đó nhận boost ×50 như active files hiện có

### REQ-004: Không mention → không đổi
> ❌ Status: Not Started

**Scenario:**
- WHEN task không chứa identifier nào khớp graph
- THEN ranking output byte-identical với trước feature (snapshot test)

## Modified Requirements

### REQ-M01: BuildRepoMap signature
> ❌ Status: Not Started

**Scenario:**
- WHEN caller không truyền task text (call-sites khác context_load nếu có)
- THEN hoạt động như cũ (tham số optional/variadic)

## Removed Requirements
- Không có.
