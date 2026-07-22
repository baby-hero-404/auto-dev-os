# Specs: Search-Replace Fuzzy Fallback

## Added Requirements

### REQ-001: Tier 0 giữ nguyên hành vi exact
> ✅ Status: Done

**Scenario:**
- WHEN search block match exact duy nhất
- THEN kết quả apply byte-identical với implementation hiện tại (toàn bộ test cũ pass không sửa)

### REQ-002: Tier 1 — trailing whitespace
> ✅ Status: Done

**Scenario:**
- WHEN search block khác content chỉ ở trailing spaces/tabs cuối dòng
- THEN patch apply thành công tại đúng vị trí
- AND các dòng không thuộc match giữ nguyên từng byte

### REQ-003: Tier 2 — relative indent
> ✅ Status: Done

**Scenario:**
- WHEN search block đúng nội dung nhưng toàn khối lệch indent đều (vd LLM bỏ 4 spaces đầu)
- THEN match được tìm theo relative indent và replace block được re-indent theo indent thực tế của content

### REQ-004: Tier 3 — line-trim
> ✅ Status: Done

**Scenario:**
- WHEN từng dòng của search match content sau TrimSpace, cùng số dòng, duy nhất 1 vị trí
- THEN apply thành công, giữ indentation của content cho phần không đổi giữa search/replace

### REQ-005: Unique-match enforcement mọi tier
> ✅ Status: Done

**Scenario:**
- WHEN một tier tìm thấy ≥2 vị trí match
- THEN fail ngay với message chứa tier name + số vị trí (không thử tier tiếp theo — nhiều match mờ là tín hiệu patch nguy hiểm)

### REQ-006: Tier telemetry
> ✅ Status: Done

**Scenario:**
- WHEN apply thành công ở tier > 0
- THEN log line ghi `search_replace tier=<n>` để đo tỷ lệ cứu patch

## Modified Requirements

### REQ-M01: Error message khi mọi tier fail
> ✅ Status: Done

**Scenario:**
- WHEN không tier nào match
- THEN error giữ thông tin cũ (search block not found) + gợi ý dòng gần giống nhất (best-effort, first 3 lines diff)

## Removed Requirements
- Không có.
