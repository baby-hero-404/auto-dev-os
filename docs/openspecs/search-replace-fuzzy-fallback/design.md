# Design: Search-Replace Fuzzy Fallback

## Pipeline

```go
// patch/fuzzy.go
type matchResult struct { start, end int; replaceText string; tier int }

func findMatch(content, search, replace string) (*matchResult, error) {
    for tier, fn := range []matcherFn{exactMatch, trailingWSMatch, relativeIndentMatch, lineTrimMatch} {
        locs := fn(content, search)
        switch len(locs) {
        case 0: continue
        case 1: return buildResult(locs[0], replace, tier), nil
        default: return nil, fmt.Errorf("search block matches %d locations at tier %d (%s)", len(locs), tier, tierNames[tier])
        }
    }
    return nil, notFoundWithHint(content, search)
}
```

`ApplySearchReplace` (search_replace.go:97) giữ nguyên signature; thân hàm gọi `findMatch` rồi splice.

## Matcher chi tiết

- **trailingWSMatch**: split lines, `strings.TrimRight(line, " \t")` cả 2 phía, join, tìm; map lại offset về content gốc bằng line index (làm việc trên line indices, không byte offsets, cho cả 3 fuzzy tiers).
- **relativeIndentMatch**: tính `commonIndent(searchLines)`, strip; slide window qua content lines, mỗi window tính commonIndent riêng, strip, so sánh. Khi match: `delta = windowIndent - searchIndent`; replace lines được thêm/bớt `delta` (chỉ với dòng non-empty).
- **lineTrimMatch**: so `TrimSpace` từng dòng. Khi replace: dòng nào xuất hiện y hệt (sau trim) trong cả search lẫn replace → giữ nguyên dòng content gốc (bảo toàn indent); dòng mới → dùng indent của dòng content liền trước.
- **notFoundWithHint**: tìm dòng đầu của search (trimmed) trong content; nếu thấy, đưa 3 dòng quanh đó vào error làm gợi ý.

## Vì sao fail-fast khi multi-match ở tier mờ

Tier càng mờ càng dễ match nhầm chỗ. Unique-match là điều kiện an toàn tối thiểu; nếu 2+ chỗ cùng match mờ thì xác suất chọn sai ~50%+ — thà fail để LLM regenerate với context lớn hơn (patch_retry_loop đã có sẵn cơ chế này).

## Testing

- Toàn bộ test hiện có của `search_replace_test.go` pass không sửa (REQ-001).
- Table-driven tests mỗi tier: match/multi-match/no-match.
- Corpus: trích 5-10 patch fail thật từ `server/.data/logs/*.jsonl` (grep "search block not found") làm regression fixtures.
