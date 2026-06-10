# Hướng Dẫn Khởi Tạo Git Personal Access Token (PAT)

Tài liệu này hướng dẫn chi tiết cách khởi tạo và cấu hình Personal Access Token (PAT) cho GitHub và GitHub Enterprise để tích hợp an toàn với hệ thống **Auto Code OS**. Token này cho phép các AI Agent thực hiện các thao tác tự động như clone repository, tạo branch, push code và tạo Pull Request (PR).

---

## 1. GitHub và GitHub Enterprise

GitHub cung cấp hai loại Personal Access Token: **Fine-grained tokens** (Khuyến khích sử dụng vì tính bảo mật cao, giới hạn phạm vi truy cập cụ thể) và **Tokens (classic)**.

Auto Code OS hiện hỗ trợ GitHub public API và GitHub Enterprise API. Với GitHub public, để trống trường **GitHub API Base URL** trong UI. Với GitHub Enterprise, nhập API base URL dạng:

```text
https://github.example.com/api/v3
```

### Cách 1: Sử dụng Fine-grained Personal Access Token (Khuyến nghị)

Fine-grained PAT cho phép bạn giới hạn quyền truy cập chỉ cho một số repository cụ thể và phân quyền chi tiết.

#### Các bước thực hiện:
1. Đăng nhập vào tài khoản GitHub của bạn.
2. Đi tới **Settings** (bằng cách nhấp vào ảnh đại diện ở góc trên bên phải -> Chọn **Settings**).
3. Ở thanh menu bên trái, cuộn xuống dưới cùng và chọn **Developer settings**.
4. Chọn **Personal access tokens** -> **Fine-grained tokens**.
5. Nhấp vào nút **Generate new token**.
6. Điền các thông tin cấu hình:
   * **Token name:** Đặt tên gợi nhớ (ví dụ: `auto-code-os-token`).
   * **Expiration:** Chọn thời gian hết hạn phù hợp (khuyên dùng từ 30 đến 90 ngày).
   * **Description:** Mô tả mục đích sử dụng.
   * **Resource owner:** Chọn tài khoản cá nhân hoặc tổ chức sở hữu repository.
7. **Repository access:**
   * Chọn **Only select repositories** và chọn đúng các repository mà bạn muốn tích hợp với Auto Code OS. (Không nên chọn *All repositories* để đảm bảo an toàn).
8. **Permissions (Quyền hạn):** Nhấp để mở rộng mục **Repository permissions** và thiết lập các quyền tối thiểu sau:
   
   | Quyền hạn (Repository Permission) | Mức độ cấp quyền (Access) | Mục đích sử dụng |
   | :--- | :--- | :--- |
   | **Contents** | **Read and write** | Clone mã nguồn, pull code mới nhất, tạo nhánh mới và push commit. |
   | **Pull requests** | **Read and write** | Tự động tạo và cập nhật Pull Request sau khi AI hoàn thành task. |
   | **Metadata** | **Read-only** | Đọc cấu hình repository (Tự động được cấp khi chọn Contents). |
   | **Administration** | **Read-only** | (Không bắt buộc) Đọc cấu trúc cơ bản của repo. |
   | **Workflows** | **Read and write** | (Tùy chọn) Chỉ cần nếu AI cần cập nhật file GitHub Actions `.github/workflows`. |

9. Cuộn xuống dưới cùng và nhấp vào nút **Generate token**.
10. **Lưu ý cực kỳ quan trọng:** Sao chép và lưu trữ token này ngay lập tức vào nơi an toàn. GitHub sẽ không hiển thị lại token này lần thứ hai.

---

### Cách 2: Sử dụng Personal Access Token Classic (Hỗ trợ dự phòng)

Sử dụng loại token này nếu bạn cần truy cập toàn bộ repository trên tài khoản cá nhân nhanh chóng, mặc dù mức độ bảo mật sẽ thấp hơn Fine-grained token.

#### Các bước thực hiện:
1. Truy cập **Settings** -> **Developer settings** -> **Personal access tokens** -> **Tokens (classic)**.
2. Chọn **Generate new token** -> **Generate new token (classic)**.
3. Cấu hình các thông tin cơ bản:
   * **Note:** Đặt tên (ví dụ: `auto-code-os-classic`).
   * **Expiration:** Chọn thời gian hết hạn.
4. **Select scopes (Chọn phạm vi quyền):**
   * Tick vào ô **`repo`** (Cung cấp toàn quyền kiểm soát repository bao gồm cả repo private và public, cũng như trạng thái commit).
   * *(Tùy chọn)* Tick vào ô **`workflow`** nếu bạn muốn cho phép AI quản lý GitHub Actions workflows.
5. Nhấp vào **Generate token** ở dưới cùng.
6. Sao chép và lưu trữ mã token an toàn.

---

## 2. GitLab (Mở rộng trong tương lai)

GitLab chưa được backend hiện tại hỗ trợ cho clone/push/PR automation. Phần này chỉ là ghi chú định hướng nếu sau này bổ sung provider GitLab.

### Các bước thực hiện:
1. Đăng nhập GitLab và đi tới **User Settings** (ảnh đại diện -> **Preferences**).
2. Chọn **Access Tokens** ở menu bên trái.
3. Điền thông tin token:
   * **Token name:** `auto-code-os-gitlab`.
   * **Expiration date:** Ngày hết hạn.
4. **Select scopes:**
   * Tick chọn **`api`** (Quyền truy cập API đầy đủ để quản lý repo, branch, merge request).
   * Hoặc chọn tối thiểu: **`read_repository`** và **`write_repository`**.
5. Chọn **Create personal access token**.
6. Sao chép token vừa hiển thị.

---

## 3. Gitea (Self-hosted - Mở rộng trong tương lai)

Gitea chưa được backend hiện tại hỗ trợ cho clone/push/PR automation. Phần này chỉ là ghi chú định hướng nếu sau này bổ sung provider Gitea.

Đối với các dự án sử dụng Gitea tự host:
1. Truy cập **Settings** (Cài đặt tài khoản) -> **Applications** (Ứng dụng).
2. Ở phần **Manage Personal Access Tokens**, nhập tên token (ví dụ: `auto-code-os-gitea`).
3. Chọn các quyền (Scopes): Chọn quyền truy cập repo và pull request tương tự GitHub.
4. Nhấp vào **Generate Token** và lưu lại mã token.

---

## 4. Hướng Dẫn Cấu Hình Trong Auto Code OS

Sau khi có Git Token, bạn tiến hành tích hợp vào hệ thống theo các bước sau:
1. Truy cập **Settings** -> **Git Accounts**.
2. Nhấp **Add Account**.
3. Nhập **Display Name**, chọn **Provider: GitHub**, và dán PAT vào trường **Personal Access Token**.
4. Nếu dùng GitHub Enterprise, nhập **GitHub API Base URL** dạng `https://github.example.com/api/v3`. Nếu dùng GitHub public, để trống trường này.
5. Nhấp **Save**, sau đó dùng nút **Test** để xác thực token.
6. Khi link repository trong Project, chọn Git Account vừa tạo ở trường **Git Account Credential**. Trường manual token chỉ cần dùng khi muốn override token của Git Account cho riêng repository đó.

## 5. Ghi chú vận hành GitOps

* Auto Code OS tự cấu hình `user.name` và `user.email` trong workspace clone trước khi commit, nên container mới không cần global Git identity.
* Nếu không có thay đổi mới trong workspace, bước commit/push sẽ bỏ qua thay vì báo lỗi `nothing to commit`.
* Danh sách repository GitHub được đọc theo pagination từ `Link` header, không chỉ giới hạn ở 100 repository đầu tiên.
* Khi tạo Pull Request thất bại, lỗi trả về sẽ gồm cả response body của GitHub để dễ chẩn đoán các trường hợp như PR đã tồn tại hoặc branch chưa push.
* Workspace theo task được reset bằng `git reset --hard` và `git clean -fdx` khi tái sử dụng thư mục đã clone, giúp tránh thay đổi cũ còn sót lại.
