package main
import (
	"fmt"
	"strings"
)

func main() {
	title := "zentao"
	desc := "# GitLab ↔ Zentao Personal Sync (MVP)\n\nXây dựng một dịch vụ đồng bộ cá nhân bằng **Go** nhằm tự động chuyển đổi hoạt động phát triển trên GitLab thành các task trên Zentao, giảm thao tác quản lý công việc hằng ngày.\n\nTrong giai đoạn MVP, hệ thống hoạt động theo mô hình **Scheduled Synchronization** thay vì Webhook/Event-driven. GitLab được xem là **Source of Truth**, service sẽ định kỳ quét commit thông qua GitLab API, lưu trạng thái đồng bộ vào **SQLite**, sau đó thực hiện các tác vụ tự động theo lịch.\n\n## Phạm vi\n\n* Sử dụng Go làm ngôn ngữ phát triển.\n* Kết nối GitLab API để lấy commit của chính người dùng.\n* Lưu commit và trạng thái đồng bộ vào SQLite.\n* Scheduler chạy theo lịch:\n\n  * **Đầu ngày:** lấy commit chưa xử lý và tự động tạo một task trên Zentao.\n  * **Cuối ngày:** tự động clone task đã tạo để chuẩn bị cho ngày làm việc tiếp theo.\n* Thiết kế theo kiến trúc module gồm:\n\n  * Scheduler\n  * GitLab Client\n  * Zentao Client\n  * Sync Engine\n  * SQLite Repository\n\nKiến trúc cần đủ linh hoạt để trong tương lai có thể bổ sung Webhook và mở rộng thành một dịch vụ đồng bộ GitLab ↔ Zentao hoàn chỉnh mà không phải thay đổi business logic hiện có."

	text := strings.ToLower(title + " " + desc)
	complexity := "easy"

	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}

	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = "hard"
			break
		}
	}
	if complexity != "hard" {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = "medium"
				break
			}
		}
	}
	fmt.Println("Result:", complexity)
}
