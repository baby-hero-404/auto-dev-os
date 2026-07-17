# Specs: Phân Tích Reference Projects

## Added Requirements

### REQ-001: Cấu trúc thư mục phân loại theo type
> ✅ Status: Done (4 subfolder đã tạo; README.md sẽ được thay mới ở REQ-005)

**Scenario:**
- WHEN người dùng mở `docs/references/`
- THEN thấy 4 subfolder: `agent-platform/`, `memory/`, `token-compression/`, `infrastructure/`
- AND mỗi subfolder chứa các file `DISCOVERY-{project}.md`
- AND có `README.md` master index ở root

### REQ-002: Clean old reports trước khi tạo mới
> ✅ Status: Done (root chỉ còn `README.md` + 4 subfolder)

**Scenario:**
- WHEN bắt đầu tạo reports mới
- THEN xóa **tất cả** file `.md` cũ ở root `docs/references/` trừ `README.md` — bao gồm cả dạng `DISCOVERY-*.md` lẫn dạng `{project}.md` không prefix (vd: `9router.md`, `aider.md`, `multica.md`, ...)
- AND `README.md` cũ được thay thế hoàn toàn bằng version mới
- AND sau cleanup, root `docs/references/` chỉ còn `README.md` + 4 subfolder

### REQ-003: Mỗi report đầy đủ 12 perspectives
> ✅ Status: Done (20 reports đầy đủ nội dung phân tích)

**Scenario:**
- WHEN mở bất kỳ report `DISCOVERY-*.md` nào
- THEN report chứa đầy đủ 12 sections: Architecture, Design Patterns, Code Quality, Interesting Techniques, Engineering Practices, Engineering Gems, Top 10 Things Worth Learning, Reading Guide, Anti-Patterns, Overall Evaluation, Best Features, Applied Takeaways
- AND ngôn ngữ là tiếng Việt
- AND mọi file/line reference đều được verify qua thực tế (không bịa path)

### REQ-004: Applied Takeaways mapping vào Auto Code OS
> ✅ Status: Done (mỗi report có Applied Takeaways với Impact/Effort/Risk/Est)

**Scenario:**
- WHEN đọc phần "Applied Takeaways" của bất kỳ report
- THEN mỗi takeaway chỉ rõ: mechanism là gì, áp dụng vào module/table/file nào của Auto Code OS
- AND có đánh giá Impact/Effort/Risk/Est time
- AND được xếp hạng theo adoption priority

### REQ-005: Master README.md tổng hợp
> ✅ Status: Done (README.md có index 20 projects + Top 10 + cross-project comparison)

**Scenario:**
- WHEN mở `docs/references/README.md`
- THEN thấy bảng index toàn bộ 20 projects với link tới report tương ứng
- AND có Top 10 Applied Takeaways tổng hợp từ tất cả reports
- AND có phân loại relevance (⭐ rating)

### REQ-006: Report viết bằng tiếng Việt
> ✅ Status: Done (tất cả report viết bằng tiếng Việt)

**Scenario:**
- WHEN đọc bất kỳ report nào
- THEN nội dung phân tích, nhận xét, đánh giá viết bằng tiếng Việt
- AND tên section headers, technical terms có thể giữ nguyên tiếng Anh
- AND code snippets, file paths giữ nguyên

### REQ-007: Đi qua lần lượt từng project
> ✅ Status: Done (20/20 projects đã phân tích, không skip)

**Scenario:**
- WHEN thực hiện phân tích
- THEN đi qua từng project trong `references/` một cách tuần tự
- AND mỗi project tạo 1 report tương ứng trong subfolder phù hợp
- AND không skip project nào

### REQ-008: Script liệt kê & phân loại reference projects
> ✅ Status: Done (script đã tồn tại)

**Scenario:**
- WHEN chạy `scripts/analyze_references.sh`
- THEN liệt kê 20 repo theo đúng 4 category (khớp với phân loại trong design.md)
- AND đánh dấu repo nào đã clone / chưa clone trong `references/`

## Modified Requirements
- Không có

## Removed Requirements
- Không có
