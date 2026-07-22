package outputfilter

// compressPaths is a deliberate no-op. design.md's "path prefix compression" (rewrite a
// repeated absolute path to a relative one after its first occurrence) would rewrite line
// content, which REQ-007 explicitly forbids ("filter chỉ xóa/gộp/đánh dấu, không rewrite nội
// dung dòng") and which the line-subsequence property test (task 1.9) enforces. Kept as a
// pipeline stage (wired for the build/test profiles per design.md) so a future content-safe
// implementation — e.g. only compressing paths inside a synthesized marker line rather than
// the original line — can be dropped in without touching Run's call sites.
func compressPaths(lines []string) []string {
	return lines
}
