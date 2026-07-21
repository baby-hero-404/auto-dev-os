# Design: RepoMap Mention Boost

## Extraction (`mentions.go`)

```go
func ExtractMentions(text string, known map[string]bool, knownFiles map[string]bool) Mentions {
    // 1. backtick spans → ident hoặc path
    // 2. regex CamelCase | snake_case (≥2 segments) | dotted.path | slash/path.ext
    // 3. lọc: idents ∈ known definitions; paths ∈ knownFiles (match theo suffix)
}
type Mentions struct { Idents map[string]bool; Files map[string]bool }
```

Aider dùng thêm quy tắc "từ xuất hiện đúng 1 lần trong repo cũng tính" — bỏ qua v1 (nhiễu cao với repo Go nhiều từ chung), chỉ lấy tín hiệu chắc: backticks, hình thái identifier, paths.

## Ranking integration

`ranking.go` hiện nhân trọng số active files (50x). Thêm cùng chỗ:

```go
switch {
case mentions.Files[edge.DstFile]: w *= 50
case mentions.Idents[edge.DstIdent]: w *= 10
}
```

(mutually-exclusive theo thứ tự ưu tiên — không stack 500x.) Personalization vector của PageRank cũng seed các file mention như active files nếu cơ chế hiện có làm vậy với active files — soi code khi implement, giữ đối xứng.

## Wiring

`context_load` step truyền `task.Title + "\n" + task.Description` vào builder; tham số variadic `WithMentions(text)` để call-sites cũ không đổi (REQ-M01).

## Testing

Fixture graph nhỏ (3-4 files, symbol tham chiếu chéo): so rank trước/sau mention; snapshot no-mention (REQ-004); extraction table-tests tiếng Việt + tiếng Anh trong description (task users viết song ngữ).
