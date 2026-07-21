# Design: Tool-Output Filtering

## Pipeline

```go
// outputfilter/filter.go
type Filter interface { Apply(lines []string) []string }

type Profile struct {
    Name    string
    Filters []Filter // thứ tự: strip → dedup → pathcompress → errorpriority(budget)
}

func Run(toolName, output string, budget int) (filtered string, stats Stats)
```

- Làm việc trên `[]string` lines (split một lần) — mọi filter là line-level, không rewrite nội dung trong dòng (REQ-007).
- `errorpriority` là filter duy nhất biết `budget` (chars); các filter khác thuần giảm nhiễu.

## Error patterns (v1)

```go
var errorLine = regexp.MustCompile(`(?i)\b(error|err!|fail(ed|ure)?|panic|fatal|exception|traceback|undefined|cannot|✗|FAIL\b)`)
```

Giữ: mọi dòng match + 2 dòng context mỗi phía (merge ranges chồng nhau) + 20 dòng đầu + 20 dòng cuối. Nếu tổng vẫn > budget → cắt bớt context trước, error lines sau cùng. Đoạn bỏ thay bằng `[... M lines omitted ...]`.

## Profiles

| Profile | strip | dedup | pathcompress | errorpriority |
|---------|-------|-------|--------------|---------------|
| `build` (run_build, run_lint) | ✓ | ✓ | ✓ | ✓ |
| `test` (test runners) | ✓ | ✓ | ✓ | ✓ |
| `diff` (git_diff, git_status) | ✓ | ✗ | ✗ | tail-cut có đánh dấu |
| `read` (file read) | ✗ | ✗ | ✗ | ✗ (bound riêng đã có) |
| `default` | ✓ | ✓ | ✗ | ✗ |

Tool khai báo qua metadata field `OutputProfile string` trên tool definition (`internal/tool/`); registry map name→profile, không đổi logic tool.

## Integration point

`toolloop.go:44-58`: trước đoạn `maxToolResultChars`, gọi `outputfilter.Run(toolName, result, maxToolResultChars)`. Hard-cut cũ giữ nguyên ngay sau — safety net (REQ-005).

## Testing

Fixtures thật: build log Go/npm có error giữa file, test output lặp, git diff, ANSI progress bar (trích từ `server/.data/logs/`). Golden tests: input → expected filtered output. Property test: filtered là subsequence của input lines + marker lines.

## Trade-offs

- Thuần Go, không LLM-summarization (claw-compactor có nhưng đắt + risk): đơn giản, deterministic, dễ debug. LLM summarize là enhancement sau nếu metrics cho thấy chưa đủ.
- Regex error detection sẽ có miss/false-positive theo ngôn ngữ — chấp nhận v1, profiles cho phép tinh chỉnh per-tool sau.
