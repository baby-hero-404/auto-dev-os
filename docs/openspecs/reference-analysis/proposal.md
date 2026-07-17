# Proposal: Phân Tích Reference Projects — Trích Xuất Bài Học Cho Auto Code OS

## Why

Auto Code OS là một AI-native SDLC platform đang phát triển. Để đạt chất lượng tốt nhất, cần nghiên cứu và học hỏi từ các dự án open-source hàng đầu trong cùng lĩnh vực. Thư mục `references/` đã clone 20 repository thuộc 4 nhóm chính:

- **Agent/SDLC platforms**: Các hệ thống quản lý agent và vòng đời phát triển phần mềm tương tự Auto Code OS
- **Memory/Knowledge**: Hệ thống bộ nhớ cho AI agents — rất cần thiết cho orchestrator
- **Token Compression**: Tối ưu chi phí LLM — bottleneck lớn nhất trong vận hành
- **Infrastructure**: Router, proxy, key management — nền tảng hạ tầng AI

**Vấn đề hiện tại**: Các repo đã clone nhưng chưa có phân tích có hệ thống. Kiến thức nằm rải rác, không có mapping cụ thể về những gì có thể áp dụng vào Auto Code OS.

**Kết quả mong muốn**: Một bộ tài liệu phân tích hoàn chỉnh theo 12 góc nhìn của `explore-codebase` skill, viết bằng tiếng Việt, với trọng tâm "áp dụng được gì cho project hiện tại".

## What Changes

### Issue 1: Tạo cấu trúc thư mục `docs/references/` theo loại project
- Tạo 4 subfolder: `agent-platform/`, `memory/`, `token-compression/`, `infrastructure/`
- Mỗi report đặt tên: `DISCOVERY-{project}.md`

### Issue 2: Phân tích từng project theo explore-codebase skill (12 perspectives)
- Survey repo: manifests, entry points, dependency direction
- Dissect: architecture, design patterns, engineering gems
- Teach: full report với Applied Takeaways mapped vào Auto Code OS
- Output ngôn ngữ: **Tiếng Việt**

### Issue 3: Tạo Master README.md tổng hợp
- Index toàn bộ 20 reports với link
- Top 10 Applied Takeaways xếp hạng theo ưu tiên áp dụng
- Bảng so sánh cross-project theo từng khía cạnh (architecture, testing, DX, etc.)

### Issue 4: Script tự động loop qua từng project
- Script `scripts/analyze_references.sh` liệt kê và phân loại các repo (đã tồn tại — chỉ cần verify hoạt động đúng với 4 category)
- Hỗ trợ chạy lần lượt từng project

### Issue 5: Cleanup reports cũ
- Root `docs/references/` hiện còn 13 report cũ dạng `{project}.md` (không có prefix `DISCOVERY-`, không nằm trong subfolder)
- Xóa toàn bộ file `.md` cũ ở root, trừ `README.md` (sẽ được ghi đè ở Issue 3)

## Capabilities

### New Capabilities
- **Reference Analysis System**: Bộ tài liệu phân tích 20 reference projects
- **Cross-Project Comparison**: Bảng so sánh kiến trúc, patterns, và kỹ thuật
- **Applied Takeaways Registry**: Danh sách ranked các ý tưởng áp dụng cho Auto Code OS
- **Analysis Script**: Script hỗ trợ loop qua từng project

### Modified Capabilities
- Không có

### Removed Capabilities
- Không có

## Impact

| Area | Files Affected |
|------|----------------|
| Documentation | `docs/references/README.md` (master index) |
| Agent Platform Reports | `docs/references/agent-platform/DISCOVERY-{project}.md` (6 projects) |
| Memory Reports | `docs/references/memory/DISCOVERY-{project}.md` (2 projects) |
| Token Compression Reports | `docs/references/token-compression/DISCOVERY-{project}.md` (6 projects) |
| Infrastructure Reports | `docs/references/infrastructure/DISCOVERY-{project}.md` (6 projects) |
| Scripts | `scripts/analyze_references.sh` (đã tồn tại — verify) |
| Cleanup | Xóa 13 report cũ dạng `{project}.md` ở root `docs/references/` |

## Open Questions
- Không có — scope đã được xác nhận qua Socratic Gate.
