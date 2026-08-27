# 记账应用增强实施计划（分类管理 · 默认当月 · 搜索修复 · UI 全面升级）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现分类管理（categories 表 + CRUD API + 管理页面）、记账页默认当月与分类下拉选择、修复关键字搜索 500，并将全站 UI 重构为 Element Plus + ECharts 的深色金色风格（含手机端响应式）。

**Architecture:** 后端沿用 handler→service→database 三层与版本化迁移（新增 migration 004 categories 表、CategoryService/CategoryHandler，挂在现有 auth 中间件组下；注册/存量用户自动补插默认分类）。前端引入 Element Plus（unplugin 按需引入）与 ECharts（echarts/core 按需注册），以覆盖 CSS 变量实现风格 C「深色高级感」（html.dark 默认），逐页替换原生组件，<768px 降级为抽屉导航 + 卡片列表。

**Tech Stack:** Go 1.25 + Gin + MySQL 5.7（迁移机制已有）；Vue 3 + Vite 5 + Element Plus + ECharts；现有 fetch 封装（`frontend/src/api/http.js`）与 JWT 流程不变。

**设计文档：** `docs/superpowers/specs/2026-08-26-categories-ui-redesign-design.md`（已与用户逐节确认）

---

## 环境与命令约定（执行前必读）

- 终端为 **PowerShell**（Windows）。MySQL 在宿主机 **3307 端口**（WSL Docker；若 Docker 未运行需先在 WSL 内 `service docker start`）。
- 后端开发地址 `:8081`，前端 dev `:5173`（vite 代理 `/api` → 8081）。
- **集成测试隔离**：`MYSQL_TEST_DSN` 指向**专用测试库**（测试会 TRUNCATE 全部表，严禁指向开发库）。一次性准备：
  ```powershell
  docker exec -it <mysql容器名> mysql -uroot -p'<密码>' -e "CREATE DATABASE IF NOT EXISTS account_service_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
  $env:MYSQL_TEST_DSN="root:<密码>@tcp(127.0.0.1:3307)/account_service_test?parseTime=true"
  ```
  （账号/密码/容器名按实际环境替换，与 `.env` 中 `MYSQL_DSN` 一致即可。）
- 不设 `MYSQL_TEST_DSN` 时 `go test ./...` 只跑单元测试（内存 fake）。
- 前端验证命令：`cd frontend; npm run build`；手测：`cd frontend; npm run dev` 后浏览器访问 `http://localhost:5173`。
- 所有 `git commit` 在仓库根目录 `f:\project\account-service` 执行。

## 任务总览

| # | 任务 | 类型 | 主要文件 |
|---|------|------|---------|
| 1 | 修复关键字搜索 500（LIKE 转义） | 后端 | internal/database/database.go |
| 2 | categories 迁移 + Category 模型 | 后端 | internal/database/database.go, internal/models/category.go |
| 3 | CategoryService + DB 实现 + 注册预置 | 后端 | internal/service/interfaces.go, internal/database/category.go, internal/database/user.go |
| 4 | CategoryHandler + 单元测试 | 后端 | internal/handlers/category.go, internal/handlers/handler_test.go |
| 5 | 路由注册 + 分类集成测试 | 后端 | main.go, internal/database/database_test.go |
| 6 | 前端依赖 + Vite 按需引入 | 前端 | frontend/package.json, frontend/vite.config.js |
| 7 | 风格 C 主题系统 | 前端 | theme.css(新), main.css, main.js, index.html |
| 8 | ECharts 公共封装 | 前端 | frontend/src/utils/chart.js(新) |
| 9 | AppLayout 重构（侧边栏 + 移动抽屉） | 前端 | frontend/src/components/AppLayout.vue, main.css |
| 10 | LoginView 重构 | 前端 | frontend/src/views/LoginView.vue |
| 11 | CategoriesView 新页面 + 路由 | 前端 | views/CategoriesView.vue(新), router/index.js, AppLayout.vue |
| 12 | RecordsView 重构（默认当月 + 分类选择） | 前端 | views/RecordsView.vue, utils/format.js, main.css |
| 13 | SummaryView 重构（迷你趋势图） | 前端 | views/SummaryView.vue |
| 14 | ReportView 重构（折线 + 环形图） | 前端 | views/ReportView.vue |
| 15 | AdminUsersView + LogsView 重构 | 前端 | views/AdminUsersView.vue, views/LogsView.vue |
| 16 | 清理旧组件/样式 + 构建验证 + 手测清单 | 收尾 | Modal.vue, Pagination.vue, main.css |

---

### Task 1: 修复关键字搜索 500（SQL LIKE 转义）

**背景：** [database.go](file:///f:/project/account-service/internal/database/database.go#L321) 中 `List()` 的 LIKE 子句用双引号字符串 `"ESCAPE '\\'"`，Go 把 `\\` 转义为单个 `\`，MySQL 收到 `ESCAPE '\'` —— 反斜杠吞掉收尾引号，语句残缺触发 Error 1064 → 500。设计文档 3.1/3.2 节。

**Files:**
- Modify: `internal/database/database.go:320-324`
- Test: `internal/database/database_test.go`

- [ ] **Step 1: 确认现有集成测试失败（复现 bug）**

```powershell
$env:MYSQL_TEST_DSN="root:<密码>@tcp(127.0.0.1:3307)/account_service_test?parseTime=true"
go test ./internal/database/ -run "TestListRecords_FilterAndSort" -v
```

Expected: **FAIL**，错误含 `Error 1064`（keyword 用例触发语法错误）。若本机无测试库则跳过本步，直接进入 Step 2。

- [ ] **Step 2: 追加边界字符集成测试（先写测试）**

在 `internal/database/database_test.go` 的 `TestListRecords_FilterAndSort` 之后追加：

```go
func TestListRecords_KeywordSpecialChars(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	recs := []*models.Record{
		{Date: "2024-01-01", AmountCents: -100, Category: "餐饮", Description: "100% 满分"},
		{Date: "2024-01-02", AmountCents: -200, Category: "交通", Description: "a_b 地铁"},
		{Date: "2024-01-03", AmountCents: -300, Category: "购物", Description: `C:\path 购物`},
		{Date: "2024-01-04", AmountCents: -400, Category: "餐饮", Description: "普通晚餐"},
	}
	for _, r := range recs {
		if err := db.Create(ctx, r, uid); err != nil {
			t.Fatalf("Create() = %v", err)
		}
	}

	params := &models.QueryParams{Page: 1, PageSize: 10, SortField: "date", SortDir: "desc"}

	// % 不作为通配符，仅匹配字面 "100%"
	params.Keyword = "100%"
	_, total, err := db.List(ctx, params, uid)
	if err != nil {
		t.Fatalf("keyword '100%%' List() = %v", err)
	}
	if total != 1 {
		t.Errorf("keyword '100%%': total=%d, want 1", total)
	}

	// _ 不作为单字符通配符
	params.Keyword = "a_b"
	_, total, err = db.List(ctx, params, uid)
	if err != nil {
		t.Fatalf("keyword 'a_b' List() = %v", err)
	}
	if total != 1 {
		t.Errorf("keyword 'a_b': total=%d, want 1", total)
	}

	// 反斜杠按字面匹配
	params.Keyword = `C:\p`
	_, total, err = db.List(ctx, params, uid)
	if err != nil {
		t.Fatalf(`keyword 'C:\p' List() = %v`, err)
	}
	if total != 1 {
		t.Errorf(`keyword 'C:\p': total=%d, want 1`, total)
	}
}
```

- [ ] **Step 3: 运行新测试，确认失败**

```powershell
go test ./internal/database/ -run "TestListRecords_KeywordSpecialChars" -v
```

Expected: **FAIL**（Error 1064）。

- [ ] **Step 4: 应用修复（双引号 → 原始字符串）**

`internal/database/database.go` 中：

```go
// 修改前：
	if params.Keyword != "" {
		where += " AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')"
		kw := "%" + escapeLike(params.Keyword) + "%"
		args = append(args, kw, kw)
	}
```

```go
// 修改后（反引号原始字符串：MySQL 收到 ESCAPE '\\'，即转义符为反斜杠）：
	if params.Keyword != "" {
		// 注意必须用原始字符串：双引号写法会把 \\ 折叠成单个 \，
		// MySQL 收到 ESCAPE '\' 后语句残缺，触发 Error 1064（关键字搜索 500 的根因）。
		where += ` AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')`
		kw := "%" + escapeLike(params.Keyword) + "%"
		args = append(args, kw, kw)
	}
```

- [ ] **Step 5: 运行测试确认通过**

```powershell
go test ./internal/database/ -run "TestListRecords" -v
go test ./...
```

Expected: 全部 **PASS**（含 `TestListRecords_FilterAndSort` 与 `TestListRecords_KeywordSpecialChars`）。

- [ ] **Step 6: Commit**

```powershell
git add internal/database/database.go internal/database/database_test.go
git commit -m "fix: keyword search 500 caused by LIKE ESCAPE backslash collapsing"
```

---

### Task 2: categories 表迁移 + Category 模型

设计文档 1.1 节。`records.category` 保持纯文本，不做外键关联。

**Files:**
- Modify: `internal/database/database.go`（migrations 数组，003 块之后）
- Create: `internal/models/category.go`

- [ ] **Step 1: 创建模型 `internal/models/category.go`**

```go
package models

import "time"

// 分类类型常量。
const (
	CategoryExpense = "expense" // 支出
	CategoryIncome  = "income"  // 收入
)

// Category 表示用户维护的记账分类。与 records.category 为纯文本弱关联：
// 删除分类不影响历史记录中已保存的分类文字。
type Category struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // expense | income
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCategoryRequest 新增分类请求。
type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}
```

- [ ] **Step 2: 追加 migration 004**

`internal/database/database.go` 的 `migrations` 数组中，`003_token_blacklist` 块之后追加：

```go
	{
		version: "004_categories",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS categories (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id BIGINT NOT NULL,
				name VARCHAR(64) NOT NULL,
				type VARCHAR(16) NOT NULL DEFAULT 'expense',
				sort_order INT NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE KEY uk_user_cat (user_id, name, type),
				CONSTRAINT fk_categories_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
	},
```

- [ ] **Step 3: 集成测试的 truncate 列表加 categories**

`internal/database/database_test.go` 的 `truncateAll`：

```go
// 修改前：
		"TRUNCATE TABLE records",
		"TRUNCATE TABLE users",
// 修改后：
		"TRUNCATE TABLE records",
		"TRUNCATE TABLE categories",
		"TRUNCATE TABLE users",
```

- [ ] **Step 4: 验证编译与存量测试**

```powershell
go build ./...
go vet ./...
go test ./...
```

Expected: 无错误（单元测试 PASS；设置 `MYSQL_TEST_DSN` 时集成测试会自动应用 004 迁移）。

- [ ] **Step 5: Commit**

```powershell
git add internal/models/category.go internal/database/database.go internal/database/database_test.go
git commit -m "feat: add categories table (migration 004) and Category model"
```

---

### Task 3: CategoryService 接口 + DB 实现 + 注册预置默认分类

设计文档 1.2/1.3 节。默认分类在 DB 层插入：`CreateUser`/`CreateFirstUser` 同事务插入；`ListCategories` 对存量用户幂等补插（INSERT IGNORE）。

**Files:**
- Modify: `internal/service/interfaces.go`
- Create: `internal/database/category.go`
- Modify: `internal/database/user.go`（CreateUser / CreateFirstUser）

- [ ] **Step 1: 扩展 service 接口**

`internal/service/interfaces.go`：import 块加 `"errors"`；在 `RecordService` 定义之后追加：

```go
// CategoryService 定义记账分类管理操作。
type CategoryService interface {
	ListCategories(ctx context.Context, userID int64) ([]*models.Category, error)
	CreateCategory(ctx context.Context, cat *models.Category, userID int64) error
	DeleteCategory(ctx context.Context, id, userID int64) error
}
```

在文件末尾 Op 常量块中追加两个常量：

```go
	OpCreateCategory = "create_category"
	OpDeleteCategory = "delete_category"
```

在 `OperationLogEntry` 结构体之后追加域错误：

```go
// ErrDuplicateCategory 表示同一用户同一类型下已存在同名分类。
var ErrDuplicateCategory = errors.New("duplicate category")
```

- [ ] **Step 2: 创建 `internal/database/category.go`**

```go
package database

import (
	"context"
	"database/sql"
	"fmt"

	"account-service/internal/models"
	"account-service/internal/service"
)

// execer 抽象 *sql.DB 与 *sql.Tx 的公共执行能力，供默认分类插入复用。
type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// defaultCategories 默认分类集合：新用户注册（同事务）与存量用户首次拉取时补插。
var defaultCategories = []struct{ Name, Type string }{
	{"餐饮", "expense"}, {"交通", "expense"}, {"购物", "expense"},
	{"居住", "expense"}, {"娱乐", "expense"}, {"医疗", "expense"},
	{"工资", "income"}, {"理财", "income"}, {"其他收入", "income"},
}

// insertDefaultCategories 在给定执行器上幂等插入默认分类（INSERT IGNORE）。
func insertDefaultCategories(ctx context.Context, q execer, userID int64) error {
	for i, d := range defaultCategories {
		if _, err := q.ExecContext(ctx,
			`INSERT IGNORE INTO categories (user_id, name, type, sort_order) VALUES (?, ?, ?, ?)`,
			userID, d.Name, d.Type, i,
		); err != nil {
			return err
		}
	}
	return nil
}

const categoryColumns = "id, user_id, name, type, sort_order, created_at"

// ListCategories 返回当前用户全部分类（按 type、sort_order、id 排序）。
// 存量用户（无分类）首次访问时自动补插默认集合（幂等）。
func (db *DB) ListCategories(ctx context.Context, userID int64) ([]*models.Category, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	if err := insertDefaultCategories(ctx, db.conn, userID); err != nil {
		return nil, err
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE user_id = ? ORDER BY type, sort_order, id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}

// CreateCategory 新增分类；同一用户同一类型下重名返回 service.ErrDuplicateCategory。
func (db *DB) CreateCategory(ctx context.Context, cat *models.Category, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	var n int
	if err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM categories WHERE user_id = ? AND name = ? AND type = ?`,
		userID, cat.Name, cat.Type,
	).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return service.ErrDuplicateCategory
	}
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO categories (user_id, name, type) VALUES (?, ?, ?)`,
		userID, cat.Name, cat.Type,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	cat.ID = id
	cat.UserID = userID
	return nil
}

// DeleteCategory 删除自己的分类；不存在或不属于当前用户返回 sql.ErrNoRows。
func (db *DB) DeleteCategory(ctx context.Context, id, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM categories WHERE id = ? AND user_id = ?`,
		id, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
```

- [ ] **Step 3: 注册时同事务预置默认分类**

`internal/database/user.go` — 整体替换 `CreateUser` 与 `CreateFirstUser` 两个函数（其余不变）：

```go
func (db *DB) CreateUser(ctx context.Context, u *models.User, passwordHash string) error {
	role := u.Role
	if role == "" {
		role = models.RoleUser
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (username, role, password_hash, totp_secret) VALUES (?, ?, ?, ?)`,
		u.Username, role, passwordHash, u.TOTPSecret,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	u.ID = id
	if err := insertDefaultCategories(ctx, tx, u.ID); err != nil {
		return err
	}
	return tx.Commit()
}
```

```go
// CreateFirstUser 原子地创建首个用户（管理员）。使用单条 INSERT ... SELECT
// 语句保证并发注册时只有一个能成功（消除 TOCTOU 竞态）。已存在用户时返回错误。
func (db *DB) CreateFirstUser(ctx context.Context, u *models.User, passwordHash string) error {
	role := u.Role
	if role == "" {
		role = models.RoleUser
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (username, role, password_hash, totp_secret)
		 SELECT ?, ?, ?, ?
		 WHERE (SELECT COUNT(*) FROM users) = 0`,
		u.Username, role, passwordHash, u.TOTPSecret,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("users already exist")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	u.ID = id
	if err := insertDefaultCategories(ctx, tx, u.ID); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 4: 验证编译**

```powershell
go build ./...
go vet ./...
go test ./...
```

Expected: 无错误（DB 实现的断言在 Task 5 的集成测试中覆盖）。

- [ ] **Step 5: Commit**

```powershell
git add internal/service/interfaces.go internal/database/category.go internal/database/user.go
git commit -m "feat: CategoryService with DB impl, default categories on register (tx) and lazy backfill"
```

---

### Task 4: CategoryHandler + 单元测试

设计文档 1.2/5 节：校验失败 400 中文消息、重名 409「分类已存在」、删除他人分类 404。测试沿用 [handler_test.go](file:///f:/project/account-service/internal/handlers/handler_test.go) 的内存 fake + `perform()` 模式。

**Files:**
- Create: `internal/handlers/category.go`
- Modify: `internal/handlers/handler_test.go`

- [ ] **Step 1: 先写 fake 与测试（追加到 handler_test.go）**

在 `fakeOpLogService` 定义之后追加：

```go
type fakeCategoryService struct {
	cats   map[int64]*models.Category
	nextID int64
}

func newFakeCategoryService() *fakeCategoryService {
	return &fakeCategoryService{cats: make(map[int64]*models.Category), nextID: 1}
}

func (f *fakeCategoryService) ListCategories(_ context.Context, _ int64) ([]*models.Category, error) {
	var list []*models.Category
	for _, c := range f.cats {
		list = append(list, c)
	}
	return list, nil
}

func (f *fakeCategoryService) CreateCategory(_ context.Context, cat *models.Category, userID int64) error {
	for _, c := range f.cats {
		if c.UserID == userID && c.Name == cat.Name && c.Type == cat.Type {
			return service.ErrDuplicateCategory
		}
	}
	cat.ID = f.nextID
	cat.UserID = userID
	f.cats[cat.ID] = cat
	f.nextID++
	return nil
}

func (f *fakeCategoryService) DeleteCategory(_ context.Context, id, _ int64) error {
	if _, ok := f.cats[id]; !ok {
		return sql.ErrNoRows
	}
	delete(f.cats, id)
	return nil
}
```

在文件测试区（`TestRecordHandler_*` 之后）追加：

```go
// ----------------------------------------------------------------------
// category handler
// ----------------------------------------------------------------------

func TestCategoryHandler_CreateAndList(t *testing.T) {
	ch := NewCategoryHandler(newFakeCategoryService(), &fakeOpLogService{})
	w := perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"宠物","type":"expense"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
	w = perform(ch.ListCategories, "GET", "/api/categories", "", setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "宠物") {
		t.Errorf("list should contain 宠物: %s", w.Body.String())
	}
}

func TestCategoryHandler_Create_Duplicate(t *testing.T) {
	ch := NewCategoryHandler(newFakeCategoryService(), &fakeOpLogService{})
	perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"餐饮","type":"expense"}`, setupAuthContext(1, "u", "user"))
	w := perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"餐饮","type":"expense"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "分类已存在") {
		t.Errorf("body should contain 分类已存在: %s", w.Body.String())
	}
}

func TestCategoryHandler_Create_Invalid(t *testing.T) {
	ch := NewCategoryHandler(newFakeCategoryService(), &fakeOpLogService{})
	// 非法 type
	w := perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"x","type":"other"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad type status = %d, want 400", w.Code)
	}
	// 空名称
	w = perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"  ","type":"expense"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("blank name status = %d, want 400", w.Code)
	}
	// 名称超长（65 个字符）
	w = perform(ch.CreateCategory, "POST", "/api/categories", `{"name":"`+strings.Repeat("猫", 65)+`","type":"expense"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("long name status = %d, want 400", w.Code)
	}
}

func TestCategoryHandler_Delete_NotFound(t *testing.T) {
	ch := NewCategoryHandler(newFakeCategoryService(), &fakeOpLogService{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/categories/999", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setupAuthContext(1, "u", "user")(c)
	ch.DeleteCategory(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认编译失败**

```powershell
go test ./internal/handlers/ -run TestCategoryHandler -v
```

Expected: **编译 FAIL**（`NewCategoryHandler` 未定义）。

- [ ] **Step 3: 实现 `internal/handlers/category.go`**

```go
package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"account-service/internal/middleware"
	"account-service/internal/models"
	"account-service/internal/service"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	cats   service.CategoryService
	logger service.OperationLogService
}

func NewCategoryHandler(cats service.CategoryService, logger service.OperationLogService) *CategoryHandler {
	return &CategoryHandler{cats: cats, logger: logger}
}

// ListCategories 当前用户全部分类（存量用户首次访问自动补插默认分类）
// GET /api/categories
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	list, err := h.cats.ListCategories(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{"data": list})
}

// CreateCategory 新增分类 POST /api/categories {name, type}
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req models.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := len([]rune(name)); n < 1 || n > maxCategoryLen {
		respondBadRequest(c, "分类名称长度须在 1~64 位之间")
		return
	}
	if req.Type != models.CategoryExpense && req.Type != models.CategoryIncome {
		respondBadRequest(c, "分类类型必须为 expense 或 income")
		return
	}
	cat := &models.Category{Name: name, Type: req.Type}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.cats.CreateCategory(ctx, cat, userID); err != nil {
		if errors.Is(err, service.ErrDuplicateCategory) {
			respondError(c, http.StatusConflict, "分类已存在")
			return
		}
		respondServerError(c)
		return
	}
	if err := h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpCreateCategory, "category", strconv.FormatInt(cat.ID, 10), name, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "create_category")
	}
	respondCreated(c, gin.H{"data": cat})
}

// DeleteCategory 删除自己的分类（不存在/他人分类返回 404）
// DELETE /api/categories/:id
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.cats.DeleteCategory(ctx, id, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(c, "分类")
			return
		}
		respondServerError(c)
		return
	}
	if err := h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpDeleteCategory, "category", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "delete_category")
	}
	respondOK(c, gin.H{"message": "已删除"})
}
```

- [ ] **Step 4: 运行测试确认通过**

```powershell
go test ./internal/handlers/ -run TestCategoryHandler -v
go test ./...
```

Expected: 全部 **PASS**。

- [ ] **Step 5: Commit**

```powershell
git add internal/handlers/category.go internal/handlers/handler_test.go
git commit -m "feat: CategoryHandler with validation, 409 on duplicate, unit tests"
```

---

### Task 5: 路由注册 + 分类集成测试

设计文档 1.2/1.3/6 节。

**Files:**
- Modify: `main.go:111-121`
- Test: `internal/database/database_test.go`

- [ ] **Step 1: 先写集成测试（追加到 database_test.go）**

```go
func TestCreateUser_InsertsDefaultCategories(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	cats, err := db.ListCategories(ctx, uid)
	if err != nil {
		t.Fatalf("ListCategories() = %v", err)
	}
	if len(cats) != 9 {
		t.Fatalf("default categories = %d, want 9", len(cats))
	}
}

func TestListCategories_EnsuresDefaultsForLegacyUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "legacy")

	// 模拟存量用户：清空其分类后首次拉取应补插默认集合
	if _, err := db.conn.Exec(`DELETE FROM categories WHERE user_id = ?`, uid); err != nil {
		t.Fatalf("clean categories: %v", err)
	}
	cats, err := db.ListCategories(ctx, uid)
	if err != nil {
		t.Fatalf("ListCategories() = %v", err)
	}
	if len(cats) != 9 {
		t.Fatalf("legacy user should get 9 default categories, got %d", len(cats))
	}
}

func TestCategoryCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	cat := &models.Category{Name: "宠物", Type: models.CategoryExpense}
	if err := db.CreateCategory(ctx, cat, uid); err != nil {
		t.Fatalf("CreateCategory() = %v", err)
	}
	if cat.ID == 0 {
		t.Fatal("CreateCategory() did not set ID")
	}

	// 同类型重名 → ErrDuplicateCategory
	if err := db.CreateCategory(ctx, &models.Category{Name: "宠物", Type: models.CategoryExpense}, uid); err != service.ErrDuplicateCategory {
		t.Errorf("duplicate = %v, want service.ErrDuplicateCategory", err)
	}
	// 不同类型同名 → 允许
	if err := db.CreateCategory(ctx, &models.Category{Name: "宠物", Type: models.CategoryIncome}, uid); err != nil {
		t.Errorf("same name different type should be allowed: %v", err)
	}

	// 删除他人分类 → sql.ErrNoRows
	uid2 := mustCreateUser(t, db, "u2")
	if err := db.DeleteCategory(ctx, cat.ID, uid2); err != sql.ErrNoRows {
		t.Errorf("delete other's category = %v, want sql.ErrNoRows", err)
	}
	// 删除自己的
	if err := db.DeleteCategory(ctx, cat.ID, uid); err != nil {
		t.Fatalf("DeleteCategory() = %v", err)
	}
}
```

`database_test.go` 的 import 块增加 `"account-service/internal/service"`。

- [ ] **Step 2: 运行集成测试（此时路由未注册，但 DB 层测试可直接验证）**

```powershell
$env:MYSQL_TEST_DSN="root:<密码>@tcp(127.0.0.1:3307)/account_service_test?parseTime=true"
go test ./internal/database/ -run "TestCategory|TestCreateUser_Inserts" -v
```

Expected: 3 个测试 **PASS**。

- [ ] **Step 3: main.go 注册路由**

`main.go` 中 auth 分组内（`recordHandler := ...` 附近）：

```go
// 修改前：
		recordHandler := handlers.NewRecordHandler(db, db)
		summaryHandler := handlers.NewSummaryHandler(db)
// 修改后：
		recordHandler := handlers.NewRecordHandler(db, db)
		summaryHandler := handlers.NewSummaryHandler(db)
		categoryHandler := handlers.NewCategoryHandler(db, db)
```

在 `auth.GET("/report", summaryHandler.Report)` 之后追加：

```go
		auth.GET("/categories", categoryHandler.ListCategories)
		auth.POST("/categories", categoryHandler.CreateCategory)
		auth.DELETE("/categories/:id", categoryHandler.DeleteCategory)
```

- [ ] **Step 4: 验证**

```powershell
go build ./...
go vet ./...
go test ./...
```

Expected: 编译通过，全部测试 PASS。可选冒烟：启动后端 `go run .`，登录后 `curl -H "Authorization: Bearer <token>" http://localhost:8081/api/categories` 返回 9 条默认分类。

- [ ] **Step 5: Commit**

```powershell
git add main.go internal/database/database_test.go
git commit -m "feat: register category routes; integration tests for CRUD and default backfill"
```

---

### Task 6: 前端依赖 + Vite 按需引入

设计文档 4.1 节。

**Files:**
- Modify: `frontend/package.json`（经 npm 自动）
- Modify: `frontend/vite.config.js`

- [ ] **Step 1: 安装依赖**

```powershell
cd f:\project\account-service\frontend
npm install element-plus @element-plus/icons-vue echarts
npm install -D unplugin-auto-import unplugin-vue-components
```

Expected: 安装成功无 peer 冲突。

- [ ] **Step 2: 配置 vite.config.js（全量替换）**

```js
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

// base 固定为 /app/，与后端 Gin 的静态托管路径一致
export default defineConfig({
  plugins: [
    vue(),
    AutoImport({ resolvers: [ElementPlusResolver()] }),
    Components({ resolvers: [ElementPlusResolver()] }),
  ],
  base: '/app/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8081',
    },
  },
})
```

- [ ] **Step 3: 验证构建**

```powershell
npm run build
```

Expected: 构建成功（生成 `auto-imports.d.ts` / `components.d.ts` 为自动产物，无需处理）。

- [ ] **Step 4: Commit**

```powershell
cd f:\project\account-service
git add frontend/package.json frontend/package-lock.json frontend/vite.config.js
git commit -m "build: add element-plus/echarts with on-demand auto import"
```

---

### Task 7: 风格 C 主题系统（深色底 + 金色渐变）

设计文档 4.2 节。深色为默认（`html.dark`），浅色作为辅助主题。**关键**：旧页面仍依赖全局 input/select 样式，必须将其收窄到 `.form-row` 作用域，否则会污染 Element Plus 输入框内部 input。

**Files:**
- Create: `frontend/src/styles/theme.css`
- Modify: `frontend/src/styles/main.css`
- Modify: `frontend/src/main.js`
- Modify: `frontend/index.html`

- [ ] **Step 1: 创建 `frontend/src/styles/theme.css`**

```css
/* 风格 C「深色高级感」：深色底 + 金色渐变点缀。
   Element Plus 暗色变量（html.dark 为默认主题）。 */
html.dark {
  --el-color-primary: #f5c451;
  --el-color-primary-light-3: color-mix(in srgb, #f5c451 70%, #0c0c10);
  --el-color-primary-light-5: color-mix(in srgb, #f5c451 50%, #0c0c10);
  --el-color-primary-light-7: color-mix(in srgb, #f5c451 30%, #0c0c10);
  --el-color-primary-light-8: color-mix(in srgb, #f5c451 20%, #0c0c10);
  --el-color-primary-light-9: color-mix(in srgb, #f5c451 10%, #0c0c10);
  --el-color-primary-dark-2: color-mix(in srgb, #f5c451 80%, #000000);
  --el-bg-color: #0c0c10;
  --el-bg-color-overlay: #16161e;
  --el-bg-color-page: #0c0c10;
  --el-fill-color-blank: #121218;
  --el-fill-color-light: #1a1a24;
  --el-fill-color: #1e1e28;
  --el-border-color: #2e2e3c;
  --el-border-color-light: #2e2e3c;
  --el-border-color-lighter: #23232e;
  --el-text-color-primary: #e8e8ee;
  --el-text-color-regular: #c8c8d4;
  --el-text-color-secondary: #8b8b98;
  --el-text-color-placeholder: #6b6b78;
  --el-success-color: #3fd98a;
  --el-danger-color: #ff7b72;
  --el-box-shadow: 0 8px 30px rgba(0, 0, 0, 0.55);
  --el-box-shadow-light: 0 4px 16px rgba(0, 0, 0, 0.45);
}

/* 浅色辅助主题（保留主题切换能力） */
html:not(.dark) {
  --el-color-primary: #d99a17;
  --el-color-primary-light-3: #e3b04b;
  --el-color-primary-light-5: #ecc478;
  --el-color-primary-light-7: #f4d9a8;
  --el-color-primary-light-8: #f8e4c0;
  --el-color-primary-light-9: #fbefdc;
  --el-color-primary-dark-2: #b57f10;
  --el-bg-color: #f5f5f8;
  --el-bg-color-page: #ececef;
  --el-fill-color-blank: #ffffff;
  --el-border-color: #dcdfe6;
  --el-text-color-primary: #1d1d24;
  --el-text-color-regular: #3a3a44;
  --el-text-color-secondary: #6b6b78;
}
```

- [ ] **Step 2: 更新 main.css 令牌**

`frontend/src/styles/main.css` 顶部两个变量块替换为（保留 `--accent` 供未迁移页面过渡使用）：

```css
:root {
  --gold: #f5c451;
  --gold-deep: #e8930c;
  --gold-gradient: linear-gradient(135deg, #f5c451, #e8930c);
  --bg: #0c0c10;
  --bg-elev: #121218;
  --bg-input: #1a1a24;
  --border: #2e2e3c;
  --text: #e8e8ee;
  --text-dim: #8b8b98;
  --accent: #f5c451;
  --accent-hover: #e8930c;
  --danger: #ff7b72;
  --success: #3fd98a;
  --income: #3fd98a;
  --expense: #ff7b72;
  --shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
  --radius: 12px;
}

body.theme-light {
  --gold: #d99a17;
  --gold-deep: #b57f10;
  --gold-gradient: linear-gradient(135deg, #e3b04b, #d99a17);
  --bg: #ececef;
  --bg-elev: #ffffff;
  --bg-input: #f0f1f4;
  --border: #dcdfe6;
  --text: #1d1d24;
  --text-dim: #6b6b78;
  --accent: #d99a17;
  --accent-hover: #b57f10;
  --danger: #d6453d;
  --success: #1f9e63;
  --income: #1f9e63;
  --expense: #d6453d;
  --shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
}
```

- [ ] **Step 3: 收窄全局 input/select 样式（避免污染 el-input）**

`main.css` 中：

```css
/* 修改前：
input,
select {
  width: 100%;
  ...
}

input:focus,
select:focus {
  border-color: var(--accent);
}

input[type='date'] {
  color-scheme: dark;
}
*/
/* 修改后：仅作用于旧页面的表单行（Element Plus 输入框不受影响） */
.form-row input,
.form-row select {
  width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-input);
  color: var(--text);
  font-size: 14px;
  outline: none;
}

.form-row input:focus,
.form-row select:focus {
  border-color: var(--accent);
}

.form-row input[type='date'] {
  color-scheme: dark;
}

body.theme-light .form-row input[type='date'] {
  color-scheme: light;
}
```

（`body.theme-light input[type='date']` 原有规则同样加 `.form-row` 前缀。）

- [ ] **Step 4: main.css 末尾追加新工具类**

```css
/* ---------- 风格 C 新增工具类（旧组件样式在全部页面迁移后统一清理） ---------- */

/* 金色渐变按钮（覆盖 el-button--primary） */
.el-button--primary {
  background: var(--gold-gradient);
  border: none;
  color: #14140a;
  font-weight: 600;
}
.el-button--primary:hover,
.el-button--primary:focus {
  background: linear-gradient(135deg, #f7d47f, #f0a52e);
  color: #14140a;
}

/* 金色渐变文字 */
.gold-text {
  background: var(--gold-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

/* 金额配色 */
.pos { color: var(--income); }
.neg { color: var(--expense); }

/* 页头 */
.page-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.page-head h3 { margin: 0; font-size: 16px; }

/* 侧边栏 el-menu（金色高亮） */
.side-menu { border-right: none; background: transparent; }
.side-menu .el-menu-item { color: var(--text-dim); border-radius: 8px; margin: 4px 8px; height: 44px; line-height: 44px; }
.side-menu .el-menu-item:hover { background: var(--bg-input); color: var(--text); }
.side-menu .el-menu-item.is-active { background: var(--gold-gradient); color: #14140a; font-weight: 600; }

/* 顶栏左区（汉堡 + 标题） */
.topbar-left { display: flex; align-items: center; gap: 10px; }

/* 汉堡按钮（仅移动端显示） */
.hamburger { display: none; background: none; border: 1px solid var(--border); border-radius: 8px; color: var(--text); font-size: 16px; width: 40px; height: 40px; cursor: pointer; }

/* 抽屉内操作区 */
.drawer-actions { display: flex; flex-direction: column; gap: 10px; margin-top: 16px; }

/* 图表容器 */
.chart-box { width: 100%; height: 320px; }
.mini-chart { width: 100%; height: 220px; margin-bottom: 12px; }

/* 分页 */
.pager { margin-top: 14px; display: flex; justify-content: flex-end; }

@media (max-width: 767px) {
  .hamburger { display: inline-flex; align-items: center; justify-content: center; }
  .desktop-actions { display: none; }
}
@media (min-width: 768px) {
  .mobile-actions { display: none; }
}
```

- [ ] **Step 5: 更新入口文件**

`frontend/index.html`：`<html lang="zh-CN">` → `<html lang="zh-CN" class="dark">`

`frontend/src/main.js` 全量替换：

```js
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import './styles/theme.css'
import './styles/main.css'

// 默认深色主题；浅色偏好提前应用避免闪烁
if (localStorage.getItem('theme') === 'light') {
  document.documentElement.classList.remove('dark')
}

createApp(App).use(router).mount('#app')
```

- [ ] **Step 6: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；浏览器打开现有页面仍可用（配色变为深底金字），无样式错乱。

- [ ] **Step 7: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/styles/theme.css frontend/src/styles/main.css frontend/src/main.js frontend/index.html
git commit -m "feat: style C dark-gold theme with Element Plus css-var overrides"
```

---

### Task 8: ECharts 公共封装

设计文档 4.1/4.5 节（按需注册、监听 resize）。

**Files:**
- Create: `frontend/src/utils/chart.js`

- [ ] **Step 1: 创建 `frontend/src/utils/chart.js`**

```js
// ECharts 公共封装：按需注册 + 金色深色主题 + 窗口 resize 自适应
import * as echarts from 'echarts/core'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

// 风格 C 图表主题（深色底 + 金色主色）
echarts.registerTheme('gold-dark', {
  backgroundColor: 'transparent',
  textStyle: { color: '#c8c8d4' },
  color: ['#f5c451', '#3fd98a', '#ff7b72', '#e8930c', '#6fb3ff', '#b48ef7'],
  legend: { textStyle: { color: '#8b8b98' } },
  categoryAxis: {
    axisLine: { lineStyle: { color: '#2e2e3c' } },
    axisLabel: { color: '#8b8b98' },
    splitLine: { show: false },
  },
  valueAxis: {
    axisLine: { lineStyle: { color: '#2e2e3c' } },
    axisLabel: { color: '#8b8b98' },
    splitLine: { lineStyle: { color: '#1e1e28' } },
  },
})

// createChart 初始化图表并监听窗口 resize；返回 { chart, destroy }
export function createChart(el) {
  const chart = echarts.init(el, 'gold-dark')
  const onResize = () => chart.resize()
  window.addEventListener('resize', onResize)
  return {
    chart,
    destroy() {
      window.removeEventListener('resize', onResize)
      chart.dispose()
    },
  }
}
```

- [ ] **Step 2: 验证构建**

```powershell
cd f:\project\account-service\frontend
npm run build
```

Expected: 构建成功（此时尚无页面引用，纯新增模块）。

- [ ] **Step 3: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/utils/chart.js
git commit -m "feat: shared echarts wrapper with gold-dark theme and resize handling"
```

---

### Task 9: AppLayout 重构（el-menu 侧边栏 + 移动抽屉 + el-dialog）

设计文档 4.4（主布局）/4.5 节。「分类」菜单项在 Task 11 随路由一起加。

**Files:**
- Modify: `frontend/src/components/AppLayout.vue`（全量替换）
- Modify: `frontend/src/styles/main.css`（响应式规则）

- [ ] **Step 1: 全量替换 `frontend/src/components/AppLayout.vue`**

```vue
<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand gold-text">💰 记账本</div>
      <el-menu :default-active="route.path" router class="side-menu">
        <el-menu-item v-for="n in navRoutes" :key="n.path" :index="n.path">{{ n.title }}</el-menu-item>
      </el-menu>
      <div class="side-foot">v2.0 · Vue3 + Go</div>
    </aside>

    <div class="main">
      <header class="topbar">
        <div class="topbar-left">
          <button class="hamburger" type="button" aria-label="菜单" @click="drawerOpen = true">☰</button>
          <h1>{{ currentTitle }}</h1>
        </div>
        <div class="actions desktop-actions">
          <el-button size="small" @click="toggleTheme">{{ isLight ? '☀️ 浅色' : '🌙 深色' }}</el-button>
          <el-button size="small" @click="openPassword">修改密码</el-button>
          <el-button size="small" @click="openTOTP">TOTP</el-button>
          <span class="user-chip">{{ user?.username }} · {{ user?.role === 'admin' ? '管理员' : '用户' }}</span>
          <el-button size="small" type="danger" plain @click="doLogout">退出</el-button>
        </div>
        <div class="actions mobile-actions">
          <span class="user-chip">{{ user?.username }}</span>
          <el-button size="small" type="danger" plain @click="doLogout">退出</el-button>
        </div>
      </header>

      <main class="content">
        <RouterView />
      </main>
    </div>

    <!-- 移动端抽屉导航 -->
    <el-drawer v-model="drawerOpen" direction="ltr" size="72%" title="💰 记账本">
      <el-menu :default-active="route.path" router class="side-menu" @select="drawerOpen = false">
        <el-menu-item v-for="n in navRoutes" :key="n.path" :index="n.path">{{ n.title }}</el-menu-item>
      </el-menu>
      <div class="drawer-actions">
        <el-button @click="toggleTheme">{{ isLight ? '☀️ 浅色' : '🌙 深色' }}</el-button>
        <el-button @click="openPassword">修改密码</el-button>
        <el-button @click="openTOTP">TOTP</el-button>
      </div>
    </el-drawer>

    <!-- 修改密码 -->
    <el-dialog v-model="pwdOpen" title="修改密码" width="420px">
      <el-form label-width="90px">
        <el-form-item label="当前密码">
          <el-input v-model="pwdForm.old_password" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwdForm.new_password" type="password" show-password autocomplete="new-password" placeholder="8~72 位，含大小写字母、数字、特殊字符" />
        </el-form-item>
      </el-form>
      <div class="msg-error" v-if="pwdError">{{ pwdError }}</div>
      <div class="msg-ok" v-if="pwdOk">{{ pwdOk }}</div>
      <template #footer>
        <el-button @click="pwdOpen = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="changePassword">确认修改</el-button>
      </template>
    </el-dialog>

    <!-- TOTP 设置 -->
    <el-dialog v-model="totpOpen" :title="user?.totp_enabled ? '关闭 TOTP' : '启用 TOTP'" width="420px">
      <template v-if="!user?.totp_enabled">
        <div v-if="totpSetup">
          <p>请用身份验证器 App 扫描二维码或手动输入密钥：</p>
          <div class="qr-box"><img :src="totpQr" alt="TOTP 二维码" /></div>
          <div class="totp-secret">{{ totpSetup.secret }}</div>
          <el-form label-width="70px" style="margin-top: 12px">
            <el-form-item label="验证码">
              <el-input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
            </el-form-item>
          </el-form>
        </div>
        <div class="msg-error" v-if="totpError">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template v-else>
        <p>关闭 TOTP 需要验证当前密码与验证码：</p>
        <el-form label-width="70px">
          <el-form-item label="当前密码">
            <el-input v-model="totpDisablePwd" type="password" show-password />
          </el-form-item>
          <el-form-item label="验证码">
            <el-input v-model="totpCode" placeholder="6 位验证码" inputmode="numeric" />
          </el-form-item>
        </el-form>
        <div class="msg-error" v-if="totpError">{{ totpError }}</div>
        <div class="msg-ok" v-if="totpOk">{{ totpOk }}</div>
      </template>
      <template #footer>
        <el-button @click="totpOpen = false">取消</el-button>
        <el-button v-if="!user?.totp_enabled" type="primary" :disabled="!totpCode" @click="enableTOTP">启用</el-button>
        <el-button v-else type="danger" plain :disabled="!totpCode || !totpDisablePwd" @click="disableTOTP">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiFetch } from '../api/http'
import { getUser, setUser, clearSession, getRefreshToken } from '../api/auth'

const route = useRoute()
const router = useRouter()

const nav = [
  { path: '/records', title: '记账' },
  { path: '/summary', title: '汇总' },
  { path: '/report', title: '报表' },
  { path: '/users', title: '用户管理', admin: true },
  { path: '/logs', title: '操作日志', admin: true },
]
const navRoutes = computed(() => nav.filter((n) => !n.admin || getUser()?.role === 'admin'))
const currentTitle = computed(() => (route.meta && route.meta.title) || '')

const user = ref(getUser())
const drawerOpen = ref(false)

// 主题（默认深色；同时驱动 Element Plus 的 html.dark）
const isLight = ref(localStorage.getItem('theme') === 'light')
function applyTheme() {
  document.documentElement.classList.toggle('dark', !isLight.value)
  document.body.classList.toggle('theme-light', isLight.value)
}
function toggleTheme() {
  isLight.value = !isLight.value
  localStorage.setItem('theme', isLight.value ? 'light' : 'dark')
  applyTheme()
}
onMounted(applyTheme)

// 刷新用户信息
async function refreshUser() {
  try {
    const data = await api('/api/auth/me')
    setUser({ id: data.id, username: data.username, role: data.role, totp_enabled: data.totp_enabled })
    user.value = getUser()
  } catch {
    /* 未登录场景忽略 */
  }
}
onMounted(refreshUser)

// 退出登录
async function doLogout() {
  try {
    await apiFetch('/api/auth/logout', {
      method: 'POST',
      body: JSON.stringify({ refresh_token: getRefreshToken() }),
    })
  } catch {
    /* 忽略登出错误，本地仍清理 */
  }
  clearSession()
  router.push({ name: 'login' })
}

// 修改密码
const pwdOpen = ref(false)
const pwdLoading = ref(false)
const pwdError = ref('')
const pwdOk = ref('')
const pwdForm = reactive({ old_password: '', new_password: '' })
function openPassword() {
  pwdError.value = ''
  pwdOk.value = ''
  pwdForm.old_password = ''
  pwdForm.new_password = ''
  pwdOpen.value = true
}
async function changePassword() {
  pwdError.value = ''
  pwdOk.value = ''
  const err = validatePassword(pwdForm.new_password)
  if (err) {
    pwdError.value = err
    return
  }
  pwdLoading.value = true
  try {
    await api('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify(pwdForm),
    })
    pwdOk.value = '密码已修改，请重新登录'
    setTimeout(() => {
      clearSession()
      router.push({ name: 'login' })
    }, 1200)
  } catch (e) {
    pwdError.value = e.message
  } finally {
    pwdLoading.value = false
  }
}

// TOTP
const totpOpen = ref(false)
const totpSetup = ref(null)
const totpQr = ref('')
const totpCode = ref('')
const totpDisablePwd = ref('')
const totpError = ref('')
const totpOk = ref('')

async function openTOTP() {
  totpError.value = ''
  totpOk.value = ''
  totpCode.value = ''
  totpDisablePwd.value = ''
  totpOpen.value = true
  if (user.value && !user.value.totp_enabled) {
    await loadTOTPSetup()
  }
}

async function loadTOTPSetup() {
  try {
    const data = await api('/api/auth/totp/setup')
    totpSetup.value = data
    const QRCode = (await import('qrcode')).default
    totpQr.value = await QRCode.toDataURL(data.url)
  } catch (e) {
    totpError.value = e.message
  }
}

async function enableTOTP() {
  totpError.value = ''
  try {
    await api('/api/auth/totp/enable', {
      method: 'POST',
      body: JSON.stringify({ secret: totpSetup.value.secret, code: totpCode.value }),
    })
    totpOk.value = 'TOTP 已启用'
    user.value = { ...user.value, totp_enabled: true }
    setUser(user.value)
    setTimeout(() => (totpOpen.value = false), 1200)
  } catch (e) {
    totpError.value = e.message
  }
}

async function disableTOTP() {
  totpError.value = ''
  try {
    await api('/api/auth/totp/disable', {
      method: 'POST',
      body: JSON.stringify({ password: totpDisablePwd.value, code: totpCode.value }),
    })
    totpOk.value = 'TOTP 已关闭'
    user.value = { ...user.value, totp_enabled: false }
    setUser(user.value)
    setTimeout(() => (totpOpen.value = false), 1200)
  } catch (e) {
    totpError.value = e.message
  }
}

// 密码强度校验（与后端一致）
function validatePassword(pwd) {
  const bytes = new TextEncoder().encode(pwd).length
  if (bytes < 8) return '密码长度不能少于 8 位'
  if (bytes > 72) return '密码过长，不能超过 72 字节'
  if (!/[A-Z]/.test(pwd)) return '密码须包含大写字母'
  if (!/[a-z]/.test(pwd)) return '密码须包含小写字母'
  if (!/\d/.test(pwd)) return '密码须包含数字'
  if (!/[^A-Za-z0-9]/.test(pwd)) return '密码须包含特殊字符'
  return ''
}

// 主题初始化 + 监听（router 守卫可能先跳转）
watch(isLight, applyTheme)
</script>
```

- [ ] **Step 2: 更新响应式规则**

`main.css` 末尾**删除**旧的 720px 媒体查询块（`.sidebar { width: 64px; ... }` 那段），替换为：

```css
/* ---------- 布局响应式：≥1024 完整侧边栏；768~1023 窄侧边栏；<768 抽屉 ---------- */
@media (max-width: 1023px) {
  .sidebar { width: 160px; }
}
@media (max-width: 767px) {
  .sidebar { display: none; }
  .topbar { padding: 0 12px; }
  .content { padding: 12px; }
}
```

- [ ] **Step 3: 验证构建 + 手测**

```powershell
cd f:\project\account-service\frontend
npm run build
npm run dev
```

Expected: 构建成功；桌面侧边栏金色高亮当前项；DevTools 切换到 <768px 视口时侧边栏消失、出现汉堡按钮、点击打开抽屉；主题切换正常（Element Plus 组件跟随深浅色）。

- [ ] **Step 4: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/components/AppLayout.vue frontend/src/styles/main.css
git commit -m "feat: redesign layout with el-menu sidebar, mobile drawer and el-dialogs"
```

---

### Task 10: LoginView 重构

设计文档 4.4（登录）节：el-card 居中 + 品牌渐变标题。逻辑保持不变。

**Files:**
- Modify: `frontend/src/views/LoginView.vue`（全量替换）

- [ ] **Step 1: 全量替换 `frontend/src/views/LoginView.vue`**

```vue
<template>
  <div class="auth-page">
    <el-card class="auth-card" shadow="always">
      <h2 class="gold-text" style="margin: 0 0 20px; text-align: center">💰 记账本</h2>

      <el-form v-if="!showRegister" label-position="top" @submit.prevent="login">
        <el-form-item label="用户名">
          <el-input v-model.trim="form.username" autocomplete="username" placeholder="用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" placeholder="密码" @keyup.enter="login" />
        </el-form-item>
        <el-form-item v-if="needsTOTP" label="TOTP 验证码">
          <el-input v-model="form.totp_code" placeholder="6 位验证码" inputmode="numeric" autocomplete="one-time-code" @keyup.enter="login" />
        </el-form-item>
        <el-button type="primary" native-type="submit" style="width: 100%" size="large" :loading="loading">
          {{ loading ? '登录中...' : '登录' }}
        </el-button>
        <div class="msg-error" v-if="error">{{ error }}</div>
      </el-form>

      <el-form v-else label-position="top" @submit.prevent="register">
        <el-form-item label="用户名（2~32 字符）">
          <el-input v-model.trim="regForm.username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码（8~72 位，含大小写字母、数字、特殊字符）">
          <el-input v-model="regForm.password" type="password" show-password autocomplete="new-password" @keyup.enter="register" />
        </el-form-item>
        <el-button type="primary" native-type="submit" style="width: 100%" size="large" :loading="loading">
          {{ loading ? '注册中...' : '注册并登录' }}
        </el-button>
        <div class="msg-error" v-if="error">{{ error }}</div>
      </el-form>

      <div class="auth-switch" v-if="showRegister || allowRegister">
        <span v-if="!showRegister">尚无账号？</span>
        <a @click="showRegister = !showRegister">{{ showRegister ? '返回登录' : '注册首个管理员账号' }}</a>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/http'
import { setTokens, setUser } from '../api/auth'

const router = useRouter()
const showRegister = ref(false)
const allowRegister = ref(false)
const loading = ref(false)
const error = ref('')
const needsTOTP = ref(false)
const form = reactive({ username: '', password: '', totp_code: '' })
const regForm = reactive({ username: '', password: '' })

async function afterAuth(data) {
  setTokens(data.token, data.refresh_token)
  if (data.user) setUser(data.user)
  router.push({ path: '/' })
}

async function login() {
  error.value = ''
  if (!form.username || !form.password) {
    error.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  try {
    const data = await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: form.username, password: form.password, totp_code: form.totp_code || undefined }),
    })
    if (data.needs_totp) {
      needsTOTP.value = true
      error.value = '请输入 TOTP 验证码'
      return
    }
    await afterAuth(data)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function register() {
  error.value = ''
  if (!regForm.username || !regForm.password) {
    error.value = '请输入用户名和密码'
    return
  }
  const bytes = new TextEncoder().encode(regForm.password).length
  if (bytes < 8 || bytes > 72 || !/[A-Z]/.test(regForm.password) || !/[a-z]/.test(regForm.password) || !/\d/.test(regForm.password) || !/[^A-Za-z0-9]/.test(regForm.password)) {
    error.value = '密码需 8~72 位且包含大小写字母、数字、特殊字符'
    return
  }
  loading.value = true
  try {
    const data = await api('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(regForm),
    })
    await afterAuth(data)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const data = await api('/api/auth/register/status')
    allowRegister.value = !!data.allow_register
  } catch {
    allowRegister.value = false
  }
})
</script>
```

- [ ] **Step 2: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；登录页为居中 el-card + 金色渐变标题；登录/注册流程正常。

- [ ] **Step 3: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/LoginView.vue
git commit -m "feat: redesign login page with el-card and gradient brand title"
```

---

### Task 11: CategoriesView 新页面 + 路由 + 菜单项

设计文档 1.4 节：支出/收入两个 tab + 列表 + 新增/删除；删除提示「历史记录中的该分类文字不受影响」。

**Files:**
- Create: `frontend/src/views/CategoriesView.vue`
- Modify: `frontend/src/router/index.js`
- Modify: `frontend/src/components/AppLayout.vue`（nav 数组）

- [ ] **Step 1: 创建 `frontend/src/views/CategoriesView.vue`**

```vue
<template>
  <div class="card">
    <div class="page-head">
      <h3>分类管理</h3>
      <el-button type="primary" @click="openAdd">＋ 新增分类</el-button>
    </div>

    <el-tabs v-model="activeTab">
      <el-tab-pane label="支出分类" name="expense">
        <el-table :data="catsOf('expense')" v-loading="loading" empty-text="暂无分类">
          <el-table-column prop="name" label="名称" />
          <el-table-column label="创建时间" width="140">
            <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="收入分类" name="income">
        <el-table :data="catsOf('income')" v-loading="loading" empty-text="暂无分类">
          <el-table-column prop="name" label="名称" />
          <el-table-column label="创建时间" width="140">
            <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>

  <el-dialog v-model="addOpen" title="新增分类" width="400px">
    <el-form label-width="64px">
      <el-form-item label="名称">
        <el-input v-model="addForm.name" maxlength="64" show-word-limit placeholder="如：餐饮" @keyup.enter="addCategory" />
      </el-form-item>
      <el-form-item label="类型">
        <el-radio-group v-model="addForm.type">
          <el-radio value="expense">支出</el-radio>
          <el-radio value="income">收入</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="addOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="addCategory">添加</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'

const activeTab = ref('expense')
const categories = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const addOpen = ref(false)
const addForm = reactive({ name: '', type: 'expense' })

function catsOf(type) {
  return categories.value.filter((c) => c.type === type)
}

async function load() {
  loading.value = true
  try {
    const data = await api('/api/categories')
    categories.value = data.data || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openAdd() {
  error.value = ''
  addForm.name = ''
  addForm.type = activeTab.value
  addOpen.value = true
}

async function addCategory() {
  error.value = ''
  const name = addForm.name.trim()
  if (!name) {
    error.value = '请填写分类名称'
    return
  }
  saving.value = true
  try {
    await api('/api/categories', { method: 'POST', body: JSON.stringify({ name, type: addForm.type }) })
    addOpen.value = false
    ElMessage.success('分类已添加')
    activeTab.value = addForm.type
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function askDelete(row) {
  try {
    await ElMessageBox.confirm(
      `删除分类「${row.name}」后，历史记录中的该分类文字不受影响。确定删除？`,
      '删除分类',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  try {
    await api('/api/categories/' + row.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
```

- [ ] **Step 2: 注册路由**

`frontend/src/router/index.js`：顶部 import 加 `import CategoriesView from '../views/CategoriesView.vue'`；children 中 `report` 之后追加：

```js
      { path: 'categories', name: 'categories', component: CategoriesView, meta: { title: '分类' } },
```

- [ ] **Step 3: 添加侧边栏菜单项**

`frontend/src/components/AppLayout.vue` 的 nav 数组，「报表」之后插入：

```js
  { path: '/categories', title: '分类' },
```

- [ ] **Step 4: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；侧边栏出现「分类」；首次进入显示 9 条默认分类（支出 6 + 收入 3）；新增/重名（提示「分类已存在」）/删除均正常。

- [ ] **Step 5: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/CategoriesView.vue frontend/src/router/index.js frontend/src/components/AppLayout.vue
git commit -m "feat: category management page with tabs, add/delete flows"
```

---

### Task 12: RecordsView 重构（默认当月 + 结余横幅 + 分类选择 + 移动卡片）

设计文档第 2 节。要点：进入页面默认 `start_date=当月1日, end_date=当月末` 且「本月」胶囊选中；手动改日期后胶囊取消；记一笔弹窗类型切换决定金额正负与分类过滤，分类选项来自 `/api/categories`。

**Files:**
- Modify: `frontend/src/utils/format.js`（全量替换）
- Modify: `frontend/src/views/RecordsView.vue`（全量替换）
- Modify: `frontend/src/styles/main.css`（追加样式）

- [ ] **Step 1: 扩展 format.js（全量替换 `frontend/src/utils/format.js`）**

```js
// 金额（分）与日期格式化工具

// 123456 -> "¥1,234.56"；负数 -> "-¥12.00"
export function formatCents(cents) {
  const n = Number(cents) || 0
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  const yuan = (abs / 100).toFixed(2)
  const [int, dec] = yuan.split('.')
  const intFmt = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return `${sign}¥${intFmt}.${dec}`
}

// 分 -> 元字符串（用于编辑输入框）
export function centsToYuan(cents) {
  const n = Number(cents) || 0
  return (n / 100).toFixed(2)
}

// 元字符串 -> 分（整数）
export function yuanToCents(yuan) {
  const n = Number(yuan)
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

// Date -> YYYY-MM-DD
function fmtYMD(d) {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

// 今天 YYYY-MM-DD
export function today() {
  return fmtYMD(new Date())
}

// 某月范围 [月初, 月末]（YYYY-MM-DD）；默认当月
export function monthRange(d = new Date()) {
  return [
    fmtYMD(new Date(d.getFullYear(), d.getMonth(), 1)),
    fmtYMD(new Date(d.getFullYear(), d.getMonth() + 1, 0)),
  ]
}

// 上月范围 [月初, 月末]
export function prevMonthRange() {
  const d = new Date()
  return monthRange(new Date(d.getFullYear(), d.getMonth() - 1, 1))
}

export function fmtDate(s) {
  return s || ''
}
```

- [ ] **Step 2: 全量替换 `frontend/src/views/RecordsView.vue`**

```vue
<template>
  <div>
    <!-- 本月结余横幅 -->
    <div class="balance-banner">
      <div class="balance-main">
        <div class="balance-label">本月结余</div>
        <div class="balance-value">{{ formatCents(monthSummary.balance_cents) }}</div>
        <div class="balance-sub">
          <span>收入 <b class="pos">{{ formatCents(monthSummary.income_cents) }}</b></span>
          <span>支出 <b class="neg">{{ formatCents(monthSummary.expense_cents) }}</b></span>
        </div>
      </div>
      <el-button type="primary" size="large" @click="openAdd">＋ 记一笔</el-button>
    </div>

    <!-- 筛选栏 -->
    <div class="card filter-bar" style="align-items: center">
      <div class="quick-pills">
        <button
          v-for="q in quickRanges"
          :key="q.key"
          type="button"
          class="pill"
          :class="{ active: quick === q.key }"
          @click="applyQuick(q.key)"
        >
          {{ q.label }}
        </button>
      </div>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 260px"
        @change="onDateChange"
      />
      <el-input
        v-model="filters.keyword"
        placeholder="搜索描述/分类"
        clearable
        style="width: 200px"
        @keyup.enter="reload(1)"
        @clear="reload(1)"
      />
      <el-button type="primary" @click="reload(1)">查询</el-button>
    </div>

    <!-- 桌面表格 -->
    <div class="card desktop-table">
      <el-table :data="list" v-loading="loading" empty-text="暂无记录" @sort-change="onSortChange">
        <el-table-column prop="date" label="日期" sortable="custom" width="120" />
        <el-table-column prop="amount" label="金额" sortable="custom" width="130">
          <template #default="{ row }">
            <span class="num" :class="row.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.amount_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" sortable="custom" width="130">
          <template #default="{ row }">
            <el-tag :type="row.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">
              {{ row.category || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)">编辑</el-button>
            <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>

    <!-- 移动端卡片列表 -->
    <div class="record-cards">
      <div v-if="!list.length" class="empty-tip">{{ loading ? '加载中...' : '暂无记录' }}</div>
      <div v-for="r in list" :key="r.id" class="record-card">
        <div class="rc-main">
          <el-tag :type="r.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">
            {{ r.category || '未分类' }}
          </el-tag>
          <span class="rc-desc">{{ r.description || '-' }}</span>
        </div>
        <div class="rc-amount" :class="r.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(r.amount_cents) }}</div>
        <div class="rc-foot">
          <span>{{ r.date }}</span>
          <span>
            <el-button size="small" text @click="openEdit(r)">编辑</el-button>
            <el-button size="small" text type="danger" @click="askDelete(r)">删除</el-button>
          </span>
        </div>
      </div>
      <el-pagination
        class="pager"
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>

    <!-- 记一笔 / 编辑 -->
    <el-dialog v-model="editOpen" :title="editingId ? '编辑记录' : '记一笔'" width="440px">
      <el-form label-width="80px">
        <el-form-item label="类型">
          <el-radio-group v-model="form.type">
            <el-radio-button value="expense">支出</el-radio-button>
            <el-radio-button value="income">收入</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="form.date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item label="金额（元）">
          <el-input-number v-model="form.amountYuan" :min="0.01" :precision="2" controls-position="right" style="width: 100%" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" filterable placeholder="选择分类" style="width: 100%">
            <el-option v-for="c in typeCategories" :key="c.id" :label="c.name" :value="c.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" maxlength="255" placeholder="备注" />
        </el-form-item>
      </el-form>
      <div class="msg-error" v-if="error">{{ error }}</div>
      <template #footer>
        <el-button @click="editOpen = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'
import { formatCents, yuanToCents, today, monthRange, prevMonthRange } from '../utils/format'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const filters = reactive({ keyword: '' })
const sortField = ref('date')
const sortDir = ref('desc')

// ---- 默认当月 + 快捷切换 ----
const quick = ref('month') // 'month' | 'last' | 'all' | ''（自定义范围）
const quickRanges = [
  { key: 'month', label: '本月' },
  { key: 'last', label: '上月' },
  { key: 'all', label: '全部' },
]
const dateRange = ref(monthRange())

function applyQuick(key) {
  quick.value = key
  if (key === 'month') dateRange.value = monthRange()
  else if (key === 'last') dateRange.value = prevMonthRange()
  else dateRange.value = null
  reload(1)
}

// 手动改日期范围后取消快捷胶囊选中态
function onDateChange() {
  quick.value = ''
  reload(1)
}

// ---- 本月结余横幅 ----
const monthSummary = reactive({ income_cents: 0, expense_cents: 0, balance_cents: 0 })
async function loadMonthSummary() {
  try {
    const now = new Date()
    const s = await api(`/api/summary/monthly?year=${now.getFullYear()}&month=${now.getMonth() + 1}`)
    monthSummary.income_cents = s.income_cents || 0
    monthSummary.expense_cents = s.expense_cents || 0
    monthSummary.balance_cents = s.balance_cents || 0
  } catch {
    /* 横幅加载失败不阻塞列表 */
  }
}

// ---- 分类（记一笔下拉选项）----
const categories = ref([])
async function loadCategories() {
  try {
    const data = await api('/api/categories')
    categories.value = data.data || []
  } catch {
    /* 分类加载失败不阻塞列表 */
  }
}

// ---- 列表 ----
async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (dateRange.value && dateRange.value[0]) params.set('start_date', dateRange.value[0])
    if (dateRange.value && dateRange.value[1]) params.set('end_date', dateRange.value[1])
    if (filters.keyword) params.set('keyword', filters.keyword)
    params.set('sort_field', sortField.value)
    params.set('sort_dir', sortDir.value)
    const data = await api('/api/records?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onSortChange({ prop, order }) {
  if (!order) {
    sortField.value = 'date'
    sortDir.value = 'desc'
  } else {
    sortField.value = prop || 'date'
    sortDir.value = order === 'ascending' ? 'asc' : 'desc'
  }
  reload(1)
}

// ---- 记一笔 / 编辑 ----
const editOpen = ref(false)
const editingId = ref(null)
const saving = ref(false)
const error = ref('')
const form = reactive({ type: 'expense', date: '', amountYuan: undefined, category: '', description: '' })

const typeCategories = computed(() => categories.value.filter((c) => c.type === form.type))

function openAdd() {
  editingId.value = null
  error.value = ''
  Object.assign(form, { type: 'expense', date: today(), amountYuan: undefined, category: '', description: '' })
  editOpen.value = true
}

function openEdit(r) {
  editingId.value = r.id
  error.value = ''
  Object.assign(form, {
    type: r.amount_cents >= 0 ? 'income' : 'expense',
    date: r.date,
    amountYuan: Math.abs(r.amount_cents) / 100,
    category: r.category || '',
    description: r.description || '',
  })
  editOpen.value = true
}

async function save() {
  error.value = ''
  if (!form.date || !form.amountYuan) {
    error.value = '请填写日期与金额'
    return
  }
  saving.value = true
  const wasEdit = !!editingId.value
  const cents = yuanToCents(form.amountYuan)
  const payload = {
    date: form.date,
    // 金额输入为正数，保存时支出取负（与 amount_cents 语义一致）
    amount_cents: form.type === 'expense' ? -cents : cents,
    category: (form.category || '').trim(),
    description: (form.description || '').trim(),
  }
  try {
    if (wasEdit) {
      await api('/api/records/' + editingId.value, { method: 'PUT', body: JSON.stringify(payload) })
    } else {
      await api('/api/records', { method: 'POST', body: JSON.stringify(payload) })
    }
    editOpen.value = false
    ElMessage.success(wasEdit ? '已更新' : '已记一笔')
    await Promise.all([reload(page.value), loadMonthSummary()])
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

async function askDelete(r) {
  try {
    await ElMessageBox.confirm('确定删除该记录吗？此操作不可撤销。', '删除记录', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await api('/api/records/' + r.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await Promise.all([reload(page.value), loadMonthSummary()])
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(() => {
  reload(1)
  loadMonthSummary()
  loadCategories()
})
</script>
```

- [ ] **Step 3: main.css 追加样式**

```css
/* ---------- 记账页（风格 C） ---------- */
.balance-banner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  background: linear-gradient(135deg, #181820, #121218);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 20px 24px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.balance-label { color: var(--text-dim); font-size: 13px; }
.balance-value {
  font-size: 34px;
  font-weight: 800;
  line-height: 1.3;
  background: var(--gold-gradient);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.balance-sub { display: flex; gap: 16px; color: var(--text-dim); font-size: 13px; margin-top: 4px; }
.balance-sub b { font-weight: 600; }

/* 快捷胶囊（横向滚动，移动端友好） */
.quick-pills { display: flex; gap: 8px; overflow-x: auto; }
.pill {
  border: 1px solid var(--border);
  background: var(--bg-input);
  color: var(--text-dim);
  border-radius: 999px;
  padding: 6px 16px;
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
  min-height: 34px;
}
.pill.active { background: var(--gold-gradient); color: #14140a; border-color: transparent; font-weight: 600; }

/* 移动端记录卡片 */
.record-cards { display: none; }
.record-card { background: var(--bg-elev); border: 1px solid var(--border); border-radius: 10px; padding: 12px; margin-bottom: 10px; }
.rc-main { display: flex; align-items: center; gap: 8px; }
.rc-desc { color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rc-amount { font-size: 18px; font-weight: 700; margin-top: 6px; }
.rc-foot { display: flex; justify-content: space-between; align-items: center; color: var(--text-dim); font-size: 12px; margin-top: 6px; }
.empty-tip { color: var(--text-dim); text-align: center; padding: 40px 0; }

@media (max-width: 767px) {
  .desktop-table { display: none; }
  .record-cards { display: block; }
  .balance-value { font-size: 28px; }
}
```

- [ ] **Step 4: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；进入记账页默认显示当月记录且「本月」胶囊选中；点「上月」/「全部」正确切换；手动改日期范围后胶囊全部取消；记一笔弹窗切换支出/收入时分类下拉只显示对应类型；保存支出后金额为负、横幅刷新；<768px 视口显示卡片列表。

- [ ] **Step 5: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/RecordsView.vue frontend/src/utils/format.js frontend/src/styles/main.css
git commit -m "feat: records page with month default, balance banner, category select and mobile cards"
```

---

### Task 13: SummaryView 重构（迷你趋势图）

设计文档 4.4（汇总）节：汇总卡片保留；每日模式附迷你趋势图。

**Files:**
- Modify: `frontend/src/views/SummaryView.vue`（全量替换）

- [ ] **Step 1: 全量替换 `frontend/src/views/SummaryView.vue`**

```vue
<template>
  <div>
    <div class="card">
      <el-tabs v-model="mode" @tab-change="load">
        <el-tab-pane label="每日" name="daily" />
        <el-tab-pane label="每月" name="monthly" />
        <el-tab-pane label="每年" name="yearly" />
      </el-tabs>

      <div class="filter-bar">
        <template v-if="mode === 'daily'">
          <el-date-picker v-model="dailyDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" @change="load" />
        </template>
        <template v-else-if="mode === 'monthly'">
          <el-date-picker v-model="monthValue" type="month" value-format="YYYY-MM" placeholder="选择月份" @change="onMonthChange" />
        </template>
        <template v-else>
          <el-input-number v-model="year" :min="2000" :max="2100" @change="load" />
        </template>
      </div>

      <div v-if="summary" class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value pos">{{ formatCents(summary.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value neg">{{ formatCents(summary.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="summary.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(summary.balance_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ summary.count }}</div>
        </div>
      </div>

      <!-- 每日模式：当日累计结余迷你趋势图 -->
      <div v-show="hasDailyRecords" ref="trendEl" class="mini-chart"></div>

      <!-- 每日明细 -->
      <el-table v-if="mode === 'daily'" :data="summary?.records || []" empty-text="暂无记录">
        <el-table-column label="金额" width="140">
          <template #default="{ row }">
            <span class="num" :class="row.amount_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.amount_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="140">
          <template #default="{ row }">
            <el-tag :type="row.amount_cents >= 0 ? 'success' : 'warning'" effect="dark" size="small">{{ row.category || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" />
      </el-table>

      <!-- 月/年分项 -->
      <el-table v-if="mode !== 'daily'" :data="summary?.breakdown || []" empty-text="暂无数据">
        <el-table-column prop="period" :label="mode === 'monthly' ? '日期' : '月份'" width="120" />
        <el-table-column label="收入" width="140">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="140">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="140">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
    </div>
    <div v-if="error" class="msg-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'
import { createChart } from '../utils/chart'

const mode = ref('daily')
const dailyDate = ref(today())
const year = ref(new Date().getFullYear())
const month = ref(new Date().getMonth() + 1)
const monthValue = ref(`${year.value}-${String(month.value).padStart(2, '0')}`)
const summary = ref(null)
const error = ref('')

const hasDailyRecords = computed(() => mode.value === 'daily' && !!(summary.value?.records?.length))

function onMonthChange(v) {
  if (!v) return
  const [y, m] = v.split('-').map(Number)
  year.value = y
  month.value = m
  load()
}

async function load() {
  error.value = ''
  summary.value = null
  try {
    if (mode.value === 'daily') {
      summary.value = await api('/api/summary/daily?date=' + dailyDate.value)
    } else if (mode.value === 'monthly') {
      summary.value = await api(`/api/summary/monthly?year=${year.value}&month=${month.value}`)
    } else {
      summary.value = await api('/api/summary/yearly?year=' + year.value)
    }
  } catch (e) {
    error.value = e.message
  }
}

// ---- 迷你趋势图：当日每笔记录后的累计结余走势 ----
const trendEl = ref(null)
let trendChart = null

function renderTrend() {
  const recs = summary.value?.records
  if (!trendEl.value || !recs?.length) return
  trendChart?.destroy()
  trendChart = createChart(trendEl.value)
  let acc = 0
  const data = recs.map((r) => {
    acc += r.amount_cents
    return acc
  })
  trendChart.chart.setOption({
    grid: { left: 80, right: 16, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatCents(v) },
    xAxis: { type: 'category', data: recs.map((_, i) => `第${i + 1}笔`) },
    yAxis: { type: 'value', axisLabel: { formatter: (v) => (v / 100).toFixed(0) } },
    series: [
      {
        name: '累计结余',
        type: 'line',
        smooth: true,
        data,
        lineStyle: { color: '#f5c451', width: 2 },
        itemStyle: { color: '#f5c451' },
        areaStyle: { color: 'rgba(245,196,81,0.12)' },
      },
    ],
  })
}

watch(hasDailyRecords, (v) => {
  if (v) nextTick(renderTrend)
})
onBeforeUnmount(() => trendChart?.destroy())

onMounted(load)
</script>
```

- [ ] **Step 2: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；每日模式有记录时显示金色累计结余折线图；每月/每年 tab 与筛选正常；窗口缩放图表自适应。

- [ ] **Step 3: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/SummaryView.vue
git commit -m "feat: summary page with el-tabs and daily cumulative-balance mini chart"
```

---

### Task 14: ReportView 重构（折线 + 环形图）

设计文档 4.4（报表）节：按日收支折线图 + 分类占比环形图；表格保留；PDF/图片导出沿用现有逻辑。

**Files:**
- Modify: `frontend/src/views/ReportView.vue`（全量替换）

- [ ] **Step 1: 全量替换 `frontend/src/views/ReportView.vue`**

```vue
<template>
  <div>
    <div class="card filter-bar" style="align-items: center">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 280px"
      />
      <el-button type="primary" @click="load">生成报表</el-button>
      <el-button :disabled="!report" @click="exportImage">导出图片</el-button>
      <el-button :disabled="!report" @click="exportPDF">导出 PDF</el-button>
    </div>

    <div v-if="report" ref="reportContent" class="report-content">
      <h3 style="margin-top: 0">报表 {{ report.start_date }} ~ {{ report.end_date }}</h3>
      <div class="summary-cards">
        <div class="summary-card">
          <div class="label">收入</div>
          <div class="value pos">{{ formatCents(report.income_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">支出</div>
          <div class="value neg">{{ formatCents(report.expense_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">结余</div>
          <div class="value" :class="report.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(report.balance_cents) }}</div>
        </div>
        <div class="summary-card">
          <div class="label">笔数</div>
          <div class="value">{{ report.count }}</div>
        </div>
      </div>

      <template v-if="report.daily.length">
        <h4>收支趋势（按日）</h4>
        <div ref="dailyChartEl" class="chart-box"></div>
      </template>

      <template v-if="expenseCategories.length">
        <h4>支出分类占比</h4>
        <div ref="catChartEl" class="chart-box"></div>
      </template>

      <h4>按日统计</h4>
      <el-table :data="dailyPage" size="small" empty-text="无数据">
        <el-table-column prop="period" label="日期" width="120" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="130">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="report.daily.length"
        :page-size="20"
        :current-page="dailyPageNo"
        @current-change="dailyPageNo = $event"
      />

      <h4>按月统计</h4>
      <el-table :data="report.monthly" size="small" empty-text="无数据">
        <el-table-column prop="period" label="月份" width="120" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="结余" width="130">
          <template #default="{ row }">
            <span :class="row.balance_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.balance_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>

      <h4>按分类统计</h4>
      <el-table :data="catPage" size="small" empty-text="无数据">
        <el-table-column prop="category" label="分类" />
        <el-table-column label="收入" width="130">
          <template #default="{ row }"><span class="pos">{{ formatCents(row.income_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="支出" width="130">
          <template #default="{ row }"><span class="neg">{{ formatCents(row.expense_cents) }}</span></template>
        </el-table-column>
        <el-table-column label="合计" width="130">
          <template #default="{ row }">
            <span :class="row.total_cents >= 0 ? 'pos' : 'neg'">{{ formatCents(row.total_cents) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="笔数" width="80" />
      </el-table>
      <el-pagination
        class="pager"
        background
        layout="total, prev, pager, next"
        :total="report.by_category.length"
        :page-size="20"
        :current-page="catPageNo"
        @current-change="catPageNo = $event"
      />
    </div>

    <div v-if="error" class="msg-error">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/http'
import { formatCents, today } from '../utils/format'
import { createChart } from '../utils/chart'

const dateRange = ref([today(), today()])
const report = ref(null)
const error = ref('')
const dailyPageNo = ref(1)
const catPageNo = ref(1)
const reportContent = ref(null)
const dailyChartEl = ref(null)
const catChartEl = ref(null)
let dailyChart = null
let catChart = null

const dailyPage = computed(() => {
  const all = report.value?.daily || []
  return all.slice((dailyPageNo.value - 1) * 20, dailyPageNo.value * 20)
})
const catPage = computed(() => {
  const all = report.value?.by_category || []
  return all.slice((catPageNo.value - 1) * 20, catPageNo.value * 20)
})
const expenseCategories = computed(() => (report.value?.by_category || []).filter((c) => c.expense_cents > 0))

async function load() {
  error.value = ''
  if (!dateRange.value || !dateRange.value[0] || !dateRange.value[1]) {
    error.value = '请选择起止日期'
    return
  }
  const [start, end] = dateRange.value
  if (start > end) {
    error.value = '开始日期不能大于结束日期'
    return
  }
  try {
    report.value = await api(`/api/report?start_date=${start}&end_date=${end}`)
    dailyPageNo.value = 1
    catPageNo.value = 1
    await nextTick()
    renderCharts()
  } catch (e) {
    error.value = e.message
  }
}

function renderCharts() {
  if (!report.value) return

  // 按日收支折线图
  if (dailyChartEl.value && report.value.daily?.length) {
    dailyChart?.destroy()
    dailyChart = createChart(dailyChartEl.value)
    dailyChart.chart.setOption({
      tooltip: { trigger: 'axis', valueFormatter: (v) => formatCents(v) },
      legend: { data: ['收入', '支出'] },
      grid: { left: 80, right: 20, top: 40, bottom: 30 },
      xAxis: { type: 'category', data: report.value.daily.map((d) => d.period) },
      yAxis: { type: 'value', axisLabel: { formatter: (v) => (v / 100).toFixed(0) } },
      series: [
        {
          name: '收入',
          type: 'line',
          smooth: true,
          data: report.value.daily.map((d) => d.income_cents),
          lineStyle: { color: '#3fd98a', width: 2 },
          itemStyle: { color: '#3fd98a' },
        },
        {
          name: '支出',
          type: 'line',
          smooth: true,
          data: report.value.daily.map((d) => d.expense_cents),
          lineStyle: { color: '#ff7b72', width: 2 },
          itemStyle: { color: '#ff7b72' },
        },
      ],
    })
  }

  // 支出分类占比环形图
  if (catChartEl.value && expenseCategories.value.length) {
    catChart?.destroy()
    catChart = createChart(catChartEl.value)
    catChart.chart.setOption({
      tooltip: {
        trigger: 'item',
        valueFormatter: (v) => formatCents(v),
      },
      legend: { bottom: 0 },
      series: [
        {
          name: '支出分类',
          type: 'pie',
          radius: ['42%', '68%'],
          data: expenseCategories.value.map((c) => ({ name: c.category || '未分类', value: Math.abs(c.expense_cents) })),
        },
      ],
    })
  }
}

onBeforeUnmount(() => {
  dailyChart?.destroy()
  catChart?.destroy()
})

async function getCanvas() {
  const html2canvas = (await import('html2canvas')).default
  return html2canvas(reportContent.value, { backgroundColor: '#0c0c10', scale: 2, useCORS: true })
}

async function exportImage() {
  try {
    const canvas = await getCanvas()
    const a = document.createElement('a')
    a.href = canvas.toDataURL('image/png')
    a.download = `报表_${dateRange.value[0]}_${dateRange.value[1]}.png`
    a.click()
  } catch (e) {
    ElMessage.error('导出图片失败: ' + e.message)
  }
}

async function exportPDF() {
  try {
    const { jsPDF } = await import('jspdf')
    const canvas = await getCanvas()
    const imgData = canvas.toDataURL('image/png')
    const pdf = new jsPDF('p', 'mm', 'a4')
    const pageWidth = pdf.internal.pageSize.getWidth()
    const pageHeight = pdf.internal.pageSize.getHeight()
    const imgWidth = pageWidth
    const imgHeight = (canvas.height * imgWidth) / canvas.width

    let heightLeft = imgHeight
    let position = 0
    pdf.addImage(imgData, 'PNG', 0, position, imgWidth, imgHeight)
    heightLeft -= pageHeight
    while (heightLeft > 0) {
      position = heightLeft - imgHeight
      pdf.addPage()
      pdf.addImage(imgData, 'PNG', 0, position, imgWidth, imgHeight)
      heightLeft -= pageHeight
    }
    pdf.save(`报表_${dateRange.value[0]}_${dateRange.value[1]}.pdf`)
  } catch (e) {
    ElMessage.error('导出 PDF 失败: ' + e.message)
  }
}
</script>
```

- [ ] **Step 2: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；生成报表后出现按日收支折线图与支出分类环形图；导出图片/PDF 包含图表；表格与分页正常。

- [ ] **Step 3: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/ReportView.vue
git commit -m "feat: report page with echarts daily line and category donut charts"
```

---

### Task 15: AdminUsersView + LogsView 重构

设计文档 4.4（用户管理/操作日志）节：el-table + el-pagination + el-tag 角色徽标；操作日志增加分类操作名称。

**Files:**
- Modify: `frontend/src/views/AdminUsersView.vue`（全量替换）
- Modify: `frontend/src/views/LogsView.vue`（全量替换）

- [ ] **Step 1: 全量替换 `frontend/src/views/AdminUsersView.vue`**

```vue
<template>
  <div class="card">
    <div class="page-head">
      <h3>用户管理</h3>
      <el-button type="primary" @click="openAdd">＋ 添加用户</el-button>
    </div>

    <el-table :data="users" v-loading="loading" empty-text="暂无用户">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'warning' : 'info'" effect="dark" size="small">
            {{ row.role === 'admin' ? '管理员' : '用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="130">
        <template #default="{ row }">{{ (row.created_at || '').slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" text @click="openEdit(row)">编辑</el-button>
          <el-button size="small" text @click="openChangePwd(row)">改密</el-button>
          <el-button size="small" text type="danger" @click="askDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <!-- 添加用户 -->
  <el-dialog v-model="addOpen" title="添加用户" width="420px">
    <el-form label-width="70px">
      <el-form-item label="用户名">
        <el-input v-model="addForm.username" placeholder="2~32 字符" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input v-model="addForm.password" type="password" show-password placeholder="8~72 位，含大小写字母、数字、特殊字符" />
      </el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="addForm.role">
          <el-radio value="user">用户</el-radio>
          <el-radio value="admin">管理员</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="addOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="addUser">添加</el-button>
    </template>
  </el-dialog>

  <!-- 编辑用户 -->
  <el-dialog v-model="editOpen" title="编辑用户" width="420px">
    <el-form label-width="70px">
      <el-form-item label="用户名">
        <el-input v-model="editForm.username" />
      </el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="editForm.role">
          <el-radio value="user">用户</el-radio>
          <el-radio value="admin">管理员</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="updateUser">保存</el-button>
    </template>
  </el-dialog>

  <!-- 修改用户密码 -->
  <el-dialog v-model="pwdOpen" :title="'修改用户密码：' + (pwdTarget?.username || '')" width="420px">
    <el-form label-width="70px">
      <el-form-item label="新密码">
        <el-input v-model="pwdForm.password" type="password" show-password placeholder="8~72 位，含大小写字母、数字、特殊字符" />
      </el-form-item>
    </el-form>
    <div class="msg-error" v-if="error">{{ error }}</div>
    <template #footer>
      <el-button @click="pwdOpen = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="changePwd">确认</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api/http'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')

const addOpen = ref(false)
const addForm = reactive({ username: '', password: '', role: 'user' })
const editOpen = ref(false)
const editForm = reactive({ id: null, username: '', role: 'user' })
const pwdOpen = ref(false)
const pwdTarget = ref(null)
const pwdForm = reactive({ password: '' })

async function load() {
  loading.value = true
  try {
    const data = await api('/api/auth/users')
    users.value = data.data || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function validatePwd(pwd) {
  const bytes = new TextEncoder().encode(pwd).length
  if (bytes < 8 || bytes > 72) return '密码长度需 8~72 字节'
  if (!/[A-Z]/.test(pwd) || !/[a-z]/.test(pwd) || !/\d/.test(pwd) || !/[^A-Za-z0-9]/.test(pwd)) {
    return '密码需包含大小写字母、数字、特殊字符'
  }
  return ''
}

function openAdd() {
  error.value = ''
  addForm.username = ''
  addForm.password = ''
  addForm.role = 'user'
  addOpen.value = true
}

async function addUser() {
  error.value = ''
  const e = validatePwd(addForm.password)
  if (e) {
    error.value = e
    return
  }
  saving.value = true
  try {
    await api('/api/auth/users', { method: 'POST', body: JSON.stringify(addForm) })
    addOpen.value = false
    ElMessage.success('用户已添加')
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

function openEdit(u) {
  error.value = ''
  editForm.id = u.id
  editForm.username = u.username
  editForm.role = u.role
  editOpen.value = true
}

async function updateUser() {
  error.value = ''
  saving.value = true
  try {
    await api('/api/auth/users/' + editForm.id, {
      method: 'PUT',
      body: JSON.stringify({ username: editForm.username, role: editForm.role }),
    })
    editOpen.value = false
    ElMessage.success('已更新')
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

function openChangePwd(u) {
  error.value = ''
  pwdTarget.value = u
  pwdForm.password = ''
  pwdOpen.value = true
}

async function changePwd() {
  error.value = ''
  const e = validatePwd(pwdForm.password)
  if (e) {
    error.value = e
    return
  }
  saving.value = true
  try {
    await api('/api/auth/users/' + pwdTarget.value.id + '/change-password', {
      method: 'POST',
      body: JSON.stringify({ password: pwdForm.password }),
    })
    pwdOpen.value = false
    ElMessage.success('密码已修改')
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

async function askDelete(u) {
  try {
    await ElMessageBox.confirm('删除用户将级联删除其全部记账记录，此操作不可撤销！', '删除用户', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await api('/api/auth/users/' + u.id, { method: 'DELETE' })
    ElMessage.success('已删除')
    await load()
  } catch (err) {
    ElMessage.error(err.message)
  }
}

onMounted(load)
</script>
```

- [ ] **Step 2: 全量替换 `frontend/src/views/LogsView.vue`**

```vue
<template>
  <div class="card">
    <div class="filter-bar" style="align-items: center">
      <el-input-number
        v-model="userId"
        :min="1"
        placeholder="用户 ID（全部）"
        controls-position="right"
        style="width: 160px"
        @keyup.enter="reload(1)"
      />
      <el-select v-model="action" placeholder="操作类型（全部）" clearable style="width: 170px" @change="reload(1)">
        <el-option v-for="(name, key) in actionNames" :key="key" :label="name" :value="key" />
      </el-select>
      <el-button type="primary" @click="reload(1)">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" empty-text="暂无日志">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="用户" width="150">
        <template #default="{ row }">{{ row.username }}（{{ row.user_id }}）</template>
      </el-table-column>
      <el-table-column label="操作" width="110">
        <template #default="{ row }">{{ row.action_name || row.action }}</template>
      </el-table-column>
      <el-table-column prop="detail" label="详情" show-overflow-tooltip />
      <el-table-column label="IP" width="130">
        <template #default="{ row }">{{ row.ip || '-' }}</template>
      </el-table-column>
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ row.created_at }}</template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      background
      layout="total, prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="reload"
    />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/http'

const actionNames = {
  login: '登录',
  logout: '退出',
  refresh: '刷新会话',
  create_record: '创建记账',
  update_record: '更新记账',
  delete_record: '删除记账',
  create_category: '新增分类',
  delete_category: '删除分类',
  add_user: '添加用户',
  update_user: '更新用户',
  delete_user: '删除用户',
  change_password: '修改密码',
  totp_enable: '启用TOTP',
  totp_disable: '关闭TOTP',
}

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const userId = ref(null)
const action = ref('')

async function reload(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const params = new URLSearchParams()
    params.set('page', page.value)
    params.set('page_size', pageSize.value)
    if (userId.value) params.set('user_id', userId.value)
    if (action.value) params.set('action', action.value)
    const data = await api('/api/auth/operation-logs?' + params.toString())
    list.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => reload(1))
</script>
```

- [ ] **Step 3: 验证构建 + 手测**

```powershell
npm run build
npm run dev
```

Expected: 构建成功；用户管理页 el-tag 角色徽标、三个对话框与删除确认正常；操作日志筛选（用户 ID / 操作类型含新增分类、删除分类）与分页正常。

- [ ] **Step 4: Commit**

```powershell
cd f:\project\account-service
git add frontend/src/views/AdminUsersView.vue frontend/src/views/LogsView.vue
git commit -m "feat: migrate admin users and logs pages to element-plus"
```

---

### Task 16: 清理旧组件与遗留样式 + 全量验证

**Files:**
- Delete: `frontend/src/components/Modal.vue`, `frontend/src/components/Pagination.vue`
- Modify: `frontend/src/styles/main.css`

- [ ] **Step 1: 确认旧组件无引用**

```powershell
cd f:\project\account-service
git grep -n "components/Modal.vue\|components/Pagination.vue\|from '../components/Modal'\|from '../components/Pagination'"
```

Expected: 无输出（所有页面已迁移）。若有输出，先处理对应引用再继续。

- [ ] **Step 2: 删除旧组件**

```powershell
git rm frontend/src/components/Modal.vue frontend/src/components/Pagination.vue
```

- [ ] **Step 3: 清理 main.css 遗留样式块**

删除以下不再被引用的选择器区块（用编辑器搜索并整块删除）：
`/* ---------- 表格 ---------- */` 区块（`.table-wrap`、`table.table`、`td.empty`）、`.btn` / `.btn-primary` / `.btn-danger` / `.btn-sm`、`/* ---------- 表单 ---------- */` 区块（`.form-row`、`.form-grid`）、`/* ---------- 弹窗 ---------- */` 区块（`.modal-*`）、`/* ---------- 分页 ---------- */` 区块（`.pagination`、`.pager-info`）、`/* ---------- Tabs ---------- */` 区块（`.tabs`）、`.badge`、`.actions-inline`、`.report-tools`、`.income` / `.expense`（保留 `.pos` / `.neg`）。

保留：`:root` 与 `body.theme-light` 令牌、`body`/`#app` 基础、`.layout`/`.sidebar`/`.brand`/`.side-foot`/`.main`/`.topbar`/`.topbar-left`/`.actions`/`.content`、`.card`、`.summary-*`、`.num`、`.msg-error`/`.msg-ok`、`.user-chip`、`.auth-*`、`.report-content`、`.qr-box`/`.totp-secret`、Task 7/9/12 新增的全部区块与响应式规则。

- [ ] **Step 4: 全量构建验证**

```powershell
cd f:\project\account-service\frontend
npm run build
```

Expected: 构建成功。后端回归：

```powershell
cd f:\project\account-service
go build ./...
go vet ./...
go test ./...
# 集成测试（可选，需已设置 MYSQL_TEST_DSN）
go test ./internal/... -v
```

Expected: 全部 PASS。

- [ ] **Step 5: 手测清单（桌面 1280px + 手机 375px 视口模拟）**

- [ ] 登录/注册/TOTP 流程正常，深色金色主题正确
- [ ] 记账页：默认当月 + 「本月」胶囊选中；上月/全部切换；手动改日期后胶囊取消；关键字搜索不再 500
- [ ] 记一笔：支出/收入切换分类过滤；保存支出为负数；横幅数字刷新
- [ ] 分类页：默认 9 条；新增/重名 409 提示/删除确认文案正确
- [ ] 汇总页：每日迷你趋势图；月/年分项表
- [ ] 报表页：折线图 + 环形图；导出图片/PDF 含图表
- [ ] 用户管理/操作日志（admin 账号）
- [ ] <768px：抽屉导航、记录卡片列表、弹窗近全宽、触控目标 ≥44px（胶囊/按钮）
- [ ] 主题切换（深色默认 ↔ 浅色），Element Plus 组件跟随

- [ ] **Step 6: 部署确认（设计文档 4.6，无代码改动）**

`cd frontend && npm run build` 产物由 Go 单二进制托管（`/app` 路径），Linux 服务器现有 compose/直接运行流程不变。

- [ ] **Step 7: Commit**

```powershell
cd f:\project\account-service
git add -A frontend/src
git commit -m "chore: remove legacy Modal/Pagination components and unused styles"
```

---

## 自查记录（Self-Review）

1. **规格覆盖**：设计文档 §1（分类模型/API/预置/页面）→ Task 2-5、11；§2（记账页改造）→ Task 12；§3（搜索修复）→ Task 1；§4.1-4.2（依赖/主题）→ Task 6-7；§4.3-4.4（组件替换/各页面）→ Task 9-15；§4.5（响应式）→ Task 7/9/12 + Task 16 手测；§4.6（部署）→ Task 16 Step 6；§5（错误处理）→ Task 4（400/409/404）+ 各页 ElMessage；§6（测试）→ Task 1/4/5 + Task 16。无遗漏。
2. **占位符扫描**：无 TBD/TODO/「稍后实现」；所有代码步骤含完整代码；环境相关凭据（数据库密码/容器名）以「按实际环境替换」标注，非计划内容缺失。
3. **类型一致性**：`service.CategoryService` 三方法签名与 `fakeCategoryService`、`database/category.go` 实现一致；`models.CategoryExpense/CategoryIncome` 在 handler 与测试中一致；`service.ErrDuplicateCategory` 定义（Task 3）与引用（Task 3/4/5）一致；前端 `createChart()` 返回 `{ chart, destroy }` 在 Task 13/14 用法一致；`monthRange()/prevMonthRange()` 定义（Task 12 Step 1）与引用一致。
