# Specs: Tool-Output Filtering

## Added Requirements

### REQ-001: Dedup dòng lặp
> ✅ Status: Done — `dedup.go`, threshold=3

**Scenario:**
- WHEN tool output chứa ≥3 dòng identical liên tiếp
- THEN output sau filter còn 1 dòng + `[repeated N times]`
- AND 2 dòng lặp không bị gộp (ngưỡng 3)

### REQ-002: Error-priority truncation
> ✅ Status: Done — `errorpriority.go`, head/tail=20 + context=2, omitted markers

**Scenario:**
- WHEN output vượt budget và chứa dòng match error patterns ở giữa
- THEN kết quả sau cắt vẫn chứa các dòng error + 2 dòng context mỗi phía + phần đầu và cuối output
- AND vị trí bị cắt đánh dấu `[... M lines omitted ...]`

**Scenario:**
- WHEN output vượt budget và không có dòng error nào
- THEN giữ đầu + cuối, cắt giữa (thay vì chỉ cắt đuôi như hiện tại)

### REQ-003: ANSI/control strip
> ✅ Status: Done — `strip.go`

**Scenario:**
- WHEN output chứa escape codes màu hoặc `\r` progress rewrites
- THEN sau filter chỉ còn text sạch, dòng progress cuối cùng được giữ

### REQ-004: Per-tool profiles
> ✅ Status: Done — `filter.go` `toolProfiles` registry (name-keyed, not per-tool-file metadata; see tasks.md 1.7 deviation)

**Scenario:**
- WHEN tool `git_diff` chạy
- THEN dedup KHÔNG được áp (mỗi dòng diff có nghĩa); chỉ strip ANSI + đánh dấu cắt đuôi khi quá dài
- AND tool không khai báo profile → default (strip + dedup, không error-priority)

### REQ-005: Hard-cut vẫn là safety net
> ✅ Status: Done — `toolloop.go` calls `outputfilter.Run` then unchanged `truncateToolResult`

**Scenario:**
- WHEN kết quả sau toàn bộ filter vẫn > `maxToolResultChars`
- THEN hard-cut hiện tại vẫn áp cuối cùng (không bao giờ gửi quá bound cũ)

### REQ-006: Metrics
> ✅ Status: Done — `slog.Info("outputfilter", ...)` logged when input >= 1KB

**Scenario:**
- WHEN filter chạy trên output ≥1KB
- THEN log line `outputfilter tool=<name> in=<X> out=<Y> saved=<Z>%`

### REQ-007: Không đổi semantics
> ✅ Status: Done — pathcompress kept as a no-op to avoid violating byte-exact line content (see tasks.md 1.5 deviation); `TestRun_LineSubsequenceInvariant` enforces it

**Scenario:**
- WHEN exit code khác 0
- THEN exit code field không bị filter chạm tới (chỉ text output bị lọc)
- AND nội dung dòng được giữ nguyên từng byte (filter chỉ xóa/gộp/đánh dấu, không rewrite nội dung dòng)

## Modified Requirements
- Không có (hard-cut giữ nguyên vị trí, thêm pipeline trước nó).

## Removed Requirements
- Không có.
