package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"account-service/internal/models"
	"account-service/internal/service"
)

func mustTime(year int) time.Time {
	return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
}

// ----------------------------------------------------------------------
// 纯单元测试（无需外部服务，`go test ./...` 始终运行）
// ----------------------------------------------------------------------

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"normal":   "normal",
		"100%":     "100\\%",
		"a_b":      "a\\_b",
		"a\\b":     "a\\\\b",
		"%\\_%":    "\\%\\\\\\_\\%",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortExpr(t *testing.T) {
	if got := sortExpr("amount"); got != "amount_cents" {
		t.Errorf("sortExpr(amount) = %q", got)
	}
	if got := sortExpr("category"); got != "category" {
		t.Errorf("sortExpr(category) = %q", got)
	}
	if got := sortExpr("created_at"); got != "created_at" {
		t.Errorf("sortExpr(created_at) = %q", got)
	}
	if got := sortExpr("date"); got != "date" {
		t.Errorf("sortExpr(date) = %q", got)
	}
	// 非法字段回退到 date
	if got := sortExpr("evil; DROP TABLE records"); got != "date" {
		t.Errorf("sortExpr(invalid) = %q, want date", got)
	}
}

func TestFmtDate(t *testing.T) {
	if got := fmtDate(2024, 1, 5); got != "2024-01-05" {
		t.Errorf("fmtDate = %q", got)
	}
}

func TestDaysInMonth(t *testing.T) {
	if daysInMonth(2024, 2) != 29 {
		t.Error("2024-02 should have 29 days")
	}
	if daysInMonth(2023, 2) != 28 {
		t.Error("2023-02 should have 28 days")
	}
	if daysInMonth(2024, 4) != 30 {
		t.Error("2024-04 should have 30 days")
	}
	if daysInMonth(2024, 1) != 31 {
		t.Error("2024-01 should have 31 days")
	}
}

func TestRequireUserID(t *testing.T) {
	if err := requireUserID(1); err != nil {
		t.Errorf("requireUserID(1) = %v", err)
	}
	if err := requireUserID(0); err != ErrUnauthorized {
		t.Errorf("requireUserID(0) = %v, want ErrUnauthorized", err)
	}
	if err := requireUserID(-1); err != ErrUnauthorized {
		t.Errorf("requireUserID(-1) = %v, want ErrUnauthorized", err)
	}
}

// 越权守卫在触碰数据库之前就会返回，因此可用空结构体验证。
func TestRecordOps_ZeroUserID_ReturnsUnauthorized(t *testing.T) {
	db := &DB{}
	ctx := context.Background()
	if err := db.Create(ctx, &models.Record{Date: "2024-01-01", AmountCents: 100}, 0); err != ErrUnauthorized {
		t.Errorf("Create(0) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.GetByID(ctx, 1, 0); err != ErrUnauthorized {
		t.Errorf("GetByID(0) = %v, want ErrUnauthorized", err)
	}
	if _, _, err := db.List(ctx, &models.QueryParams{Page: 1, PageSize: 10}, 0); err != ErrUnauthorized {
		t.Errorf("List(0) = %v, want ErrUnauthorized", err)
	}
	if err := db.Update(ctx, 1, 0, &models.UpdateRecordRequest{}); err != ErrUnauthorized {
		t.Errorf("Update(0) = %v, want ErrUnauthorized", err)
	}
	if err := db.Delete(ctx, 1, 0); err != ErrUnauthorized {
		t.Errorf("Delete(0) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.DailySummary(ctx, "2024-01-01", 0); err != ErrUnauthorized {
		t.Errorf("DailySummary(0) = %v, want ErrUnauthorized", err)
	}
	if err := db.SaveRefreshToken(ctx, 0, "x", mustTime(2025)); err != ErrUnauthorized {
		t.Errorf("SaveRefreshToken(0) = %v, want ErrUnauthorized", err)
	}
}

// ----------------------------------------------------------------------
// MySQL 集成测试（设置 MYSQL_TEST_DSN 后运行）
// ----------------------------------------------------------------------

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN 未设置，跳过 MySQL 集成测试")
	}
	db, err := New(dsn)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	truncateAll(t, db)
	return db
}

func truncateAll(t *testing.T, db *DB) {
	t.Helper()
	stmts := []string{
		"SET FOREIGN_KEY_CHECKS=0",
		"TRUNCATE TABLE token_blacklist",
		"TRUNCATE TABLE refresh_tokens",
		"TRUNCATE TABLE operation_logs",
		"TRUNCATE TABLE login_logs",
		"TRUNCATE TABLE records",
		"TRUNCATE TABLE categories",
		"TRUNCATE TABLE users",
		"SET FOREIGN_KEY_CHECKS=1",
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			t.Fatalf("truncate %q: %v", s, err)
		}
	}
}

func mustCreateUser(t *testing.T, db *DB, username string) int64 {
	t.Helper()
	u := &models.User{Username: username, Role: models.RoleUser}
	if err := db.CreateUser(context.Background(), u, "hash"); err != nil {
		t.Fatalf("CreateUser(%s) = %v", username, err)
	}
	return u.ID
}

func TestCreateAndGetRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	r := &models.Record{Date: "2024-01-15", AmountCents: -5000, Category: "餐饮", Description: "午餐"}
	if err := db.Create(ctx, r, uid); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if r.ID == 0 {
		t.Fatal("Create() did not set ID")
	}
	got, err := db.GetByID(ctx, r.ID, uid)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}
	if got == nil || got.AmountCents != -5000 || got.UserID != uid {
		t.Errorf("GetByID() = %+v", got)
	}
	// 其他用户看不到该记录
	uid2 := mustCreateUser(t, db, "u2")
	other, _ := db.GetByID(ctx, r.ID, uid2)
	if other != nil {
		t.Error("another user should not see the record")
	}
}

func TestListRecords_FilterAndSort(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	recs := []*models.Record{
		{Date: "2024-01-01", AmountCents: 10000, Category: "工资", Description: "月薪"},
		{Date: "2024-01-02", AmountCents: -3000, Category: "餐饮", Description: "晚餐"},
		{Date: "2024-01-03", AmountCents: -2000, Category: "交通", Description: "地铁"},
	}
	for _, r := range recs {
		if err := db.Create(ctx, r, uid); err != nil {
			t.Fatalf("Create() = %v", err)
		}
	}

	params := &models.QueryParams{Page: 1, PageSize: 10, SortField: "date", SortDir: "desc"}
	list, total, err := db.List(ctx, params, uid)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("total=%d len=%d, want 3/3", total, len(list))
	}

	params.Keyword = "餐饮"
	_, total, err = db.List(ctx, params, uid)
	if err != nil || total != 1 {
		t.Errorf("keyword filter: total=%d err=%v, want 1", total, err)
	}
}

func TestListRecords_KeywordSpecialChars(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	recs := []*models.Record{
		{Date: "2024-01-01", AmountCents: -100, Category: "餐饮", Description: "100% 满分"},
		{Date: "2024-01-02", AmountCents: -200, Category: "交通", Description: "a_b 地铁"},
		{Date: "2024-01-03", AmountCents: -300, Category: "购物", Description: `C:\path 购物`},
		{Date: "2024-01-04", AmountCents: -400, Category: "餐饮", Description: "普通晚餐"},
		// 干扰行：不含字面 "100%"，若 % 未逃逸会被 pattern %100%% 误匹配
		{Date: "2024-01-05", AmountCents: -500, Category: "餐饮", Description: "报销 100 元"},
		// 干扰行：不含字面 "a_b"，若 _ 未逃逸会被 pattern %a_b% 当单字符通配符误匹配
		{Date: "2024-01-06", AmountCents: -600, Category: "交通", Description: "axb 公交"},
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

func TestUpdateDeleteRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	r := &models.Record{Date: "2024-01-10", AmountCents: -5000, Category: "餐饮"}
	if err := db.Create(ctx, r, uid); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	newDesc := "晚餐"
	if err := db.Update(ctx, r.ID, uid, &models.UpdateRecordRequest{Description: &newDesc}); err != nil {
		t.Fatalf("Update() = %v", err)
	}
	got, _ := db.GetByID(ctx, r.ID, uid)
	if got == nil || got.Description != "晚餐" {
		t.Errorf("after update: %+v", got)
	}
	if err := db.Delete(ctx, r.ID, uid); err != nil {
		t.Fatalf("Delete() = %v", err)
	}
	if got, _ := db.GetByID(ctx, r.ID, uid); got != nil {
		t.Error("record should be gone after delete")
	}
	if err := db.Update(ctx, 999999, uid, &models.UpdateRecordRequest{}); err != sql.ErrNoRows {
		t.Errorf("Update(999999) = %v, want sql.ErrNoRows", err)
	}
}

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	n, _ := db.UserCount(ctx)
	if n != 0 {
		t.Errorf("UserCount() = %d, want 0", n)
	}
	u := &models.User{Username: "admin", Role: models.RoleAdmin}
	if err := db.CreateFirstUser(ctx, u, "hash"); err != nil {
		t.Fatalf("CreateFirstUser() = %v", err)
	}
	if u.ID == 0 {
		t.Fatal("CreateFirstUser() did not set ID")
	}
	// 再次创建首个用户应失败
	if err := db.CreateFirstUser(ctx, &models.User{Username: "x", Role: models.RoleUser}, "hash"); err == nil {
		t.Error("CreateFirstUser() should fail when users exist")
	}
	if _, err := db.GetUserByUsername(ctx, "admin"); err != nil {
		t.Errorf("GetUserByUsername = %v", err)
	}
	if err := db.UpdateUser(ctx, u.ID, "admin2", models.RoleUser); err != nil {
		t.Fatalf("UpdateUser() = %v", err)
	}
	if err := db.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser() = %v", err)
	}
	if got, err := db.GetUserByID(ctx, u.ID); err != nil || got != nil {
		t.Errorf("GetUserByID after delete = %v, %v, want nil/nil", got, err)
	}
}

func TestSummariesAndReport(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	db.Create(ctx, &models.Record{Date: "2024-01-01", AmountCents: 100000, Category: "工资"}, uid)
	db.Create(ctx, &models.Record{Date: "2024-01-01", AmountCents: -20000, Category: "购物"}, uid)
	db.Create(ctx, &models.Record{Date: "2024-01-15", AmountCents: -30000, Category: "餐饮"}, uid)

	s, err := db.DailySummary(ctx, "2024-01-01", uid)
	if err != nil {
		t.Fatalf("DailySummary() = %v", err)
	}
	if s.IncomeCents != 100000 || s.ExpenseCents != 20000 || s.BalanceCents != 80000 || s.Count != 2 {
		t.Errorf("daily = %+v", s)
	}

	ms, err := db.MonthlySummary(ctx, 2024, 1, uid)
	if err != nil {
		t.Fatalf("MonthlySummary() = %v", err)
	}
	if ms.IncomeCents != 100000 || ms.ExpenseCents != 50000 || len(ms.Breakdown) != 2 {
		t.Errorf("monthly = %+v", ms)
	}

	ys, err := db.YearlySummary(ctx, 2024, uid)
	if err != nil {
		t.Fatalf("YearlySummary() = %v", err)
	}
	if ys.IncomeCents != 100000 || ys.ExpenseCents != 50000 || len(ys.Breakdown) != 1 {
		t.Errorf("yearly = %+v", ys)
	}

	rep, err := db.Report(ctx, "2024-01-01", "2024-01-31", uid)
	if err != nil {
		t.Fatalf("Report() = %v", err)
	}
	if rep.IncomeCents != 100000 || rep.ExpenseCents != 50000 || len(rep.ByCategory) != 3 {
		t.Errorf("report = %+v", rep)
	}

	// 其他用户看不到数据
	uid2 := mustCreateUser(t, db, "u2")
	other, err := db.MonthlySummary(ctx, 2024, 1, uid2)
	if err != nil {
		t.Fatalf("other user monthly = %v", err)
	}
	if other.Count != 0 {
		t.Errorf("other user should see 0 records, got %d", other.Count)
	}
}

func TestRefreshTokenLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	uid := mustCreateUser(t, db, "u1")

	if err := db.SaveRefreshToken(ctx, uid, "hash1", mustTime(2030)); err != nil {
		t.Fatalf("SaveRefreshToken() = %v", err)
	}
	got, err := db.GetRefreshToken(ctx, "hash1")
	if err != nil || got != uid {
		t.Fatalf("GetRefreshToken() = %d, %v, want %d", got, err, uid)
	}

	// 过期 token 视为无效
	if err := db.SaveRefreshToken(ctx, uid, "hash2", mustTime(2020)); err != nil {
		t.Fatalf("SaveRefreshToken(expired) = %v", err)
	}
	if got, _ := db.GetRefreshToken(ctx, "hash2"); got != 0 {
		t.Errorf("expired token should be invalid, got user %d", got)
	}

	if err := db.RevokeRefreshToken(ctx, "hash1"); err != nil {
		t.Fatalf("RevokeRefreshToken() = %v", err)
	}
	if got, _ := db.GetRefreshToken(ctx, "hash1"); got != 0 {
		t.Errorf("revoked token should be invalid, got user %d", got)
	}

	if err := db.SaveRefreshToken(ctx, uid, "hash3", mustTime(2030)); err != nil {
		t.Fatalf("SaveRefreshToken() = %v", err)
	}
	if err := db.RevokeAllRefreshTokensForUser(ctx, uid); err != nil {
		t.Fatalf("RevokeAllRefreshTokensForUser() = %v", err)
	}
	if got, _ := db.GetRefreshToken(ctx, "hash3"); got != 0 {
		t.Errorf("token should be revoked after RevokeAll, got user %d", got)
	}
}

func TestBlacklistLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// 未拉黑 -> false
	ok, err := db.IsTokenBlacklisted(ctx, "nope")
	if err != nil || ok {
		t.Fatalf("IsTokenBlacklisted(unlisted) = %v, %v, want false/nil", ok, err)
	}

	// 拉黑（未来过期）-> true
	if err := db.BlacklistToken(ctx, "hash1", mustTime(2030)); err != nil {
		t.Fatalf("BlacklistToken() = %v", err)
	}
	ok, err = db.IsTokenBlacklisted(ctx, "hash1")
	if err != nil || !ok {
		t.Fatalf("IsTokenBlacklisted(listed) = %v, %v, want true/nil", ok, err)
	}

	// 幂等：重复拉黑不报错
	if err := db.BlacklistToken(ctx, "hash1", mustTime(2030)); err != nil {
		t.Fatalf("BlacklistToken(idempotent) = %v", err)
	}

	// 已过期记录视为不在黑名单
	if err := db.BlacklistToken(ctx, "expired", mustTime(2020)); err != nil {
		t.Fatalf("BlacklistToken(expired) = %v", err)
	}
	ok, err = db.IsTokenBlacklisted(ctx, "expired")
	if err != nil || ok {
		t.Fatalf("IsTokenBlacklisted(expired) = %v, %v, want false/nil", ok, err)
	}
}

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

	// 删除一条默认分类后不复活（COUNT 守卫语义锚定）：
	// 此时 uid 分类 = 9 默认 + 宠物(income) = 10；删除「餐饮」→ 9，且不再补插。
	cats, err := db.ListCategories(ctx, uid)
	if err != nil {
		t.Fatalf("ListCategories() = %v", err)
	}
	if len(cats) != 10 {
		t.Fatalf("before deleting default: categories = %d, want 10", len(cats))
	}
	var foodID int64
	for _, c := range cats {
		if c.Name == "餐饮" {
			foodID = c.ID
		}
	}
	if foodID == 0 {
		t.Fatal("default category 餐饮 not found")
	}
	if err := db.DeleteCategory(ctx, foodID, uid); err != nil {
		t.Fatalf("DeleteCategory(餐饮) = %v", err)
	}
	cats, err = db.ListCategories(ctx, uid)
	if err != nil {
		t.Fatalf("ListCategories() after delete = %v", err)
	}
	if len(cats) != 9 {
		t.Fatalf("after deleting default: categories = %d, want 9 (deleted default must not revive)", len(cats))
	}
	hasFood, hasPetIncome := false, false
	for _, c := range cats {
		if c.Name == "餐饮" {
			hasFood = true
		}
		if c.Name == "宠物" && c.Type == models.CategoryIncome {
			hasPetIncome = true
		}
	}
	if hasFood {
		t.Error("deleted default category 餐饮 should not revive")
	}
	if !hasPetIncome {
		t.Error("category 宠物(income) should still exist")
	}
	// 再次拉取仍不复活
	cats, err = db.ListCategories(ctx, uid)
	if err != nil {
		t.Fatalf("ListCategories() second pass = %v", err)
	}
	if len(cats) != 9 {
		t.Fatalf("second ListCategories() = %d, want 9", len(cats))
	}
}
