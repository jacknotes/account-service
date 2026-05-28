package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"account-service/internal/models"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndGetRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	r := &models.Record{Date: "2024-01-15", Amount: -50.0, Category: "餐饮", Description: "午餐"}
	if err := db.Create(ctx, r, 1); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if r.ID == 0 {
		t.Fatal("Create() did not set ID")
	}

	got, err := db.GetByID(ctx, r.ID, 1)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}
	if got == nil {
		t.Fatal("GetByID() returned nil")
	}
	if got.Date != "2024-01-15" || got.Amount != -50.0 {
		t.Errorf("GetByID() = %+v, want date=2024-01-15 amount=-50.0", got)
	}
}

func TestListRecords(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	records := []*models.Record{
		{Date: "2024-01-01", Amount: 100, Category: "工资", Description: "月薪"},
		{Date: "2024-01-02", Amount: -30, Category: "餐饮", Description: "晚餐"},
		{Date: "2024-01-03", Amount: -20, Category: "交通", Description: "地铁"},
	}
	for _, r := range records {
		if err := db.Create(ctx, r, 1); err != nil {
			t.Fatalf("Create() = %v", err)
		}
	}

	params := &models.QueryParams{Page: 1, PageSize: 10}
	list, total, err := db.List(ctx, params, 1)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(list) != 3 {
		t.Errorf("len(list) = %d, want 3", len(list))
	}

	// keyword filter
	params.Keyword = "餐饮"
	list, total, err = db.List(ctx, params, 1)
	if err != nil {
		t.Fatalf("List(keyword) = %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("keyword filter: total=%d len=%d, want 1", total, len(list))
	}
}

func TestUpdateRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	r := &models.Record{Date: "2024-01-10", Amount: -50, Category: "餐饮", Description: "午餐"}
	if err := db.Create(ctx, r, 1); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	newDesc := "晚餐"
	req := &models.UpdateRecordRequest{Description: &newDesc}
	if err := db.Update(ctx, r.ID, 1, req); err != nil {
		t.Fatalf("Update() = %v", err)
	}

	got, err := db.GetByID(ctx, r.ID, 1)
	if err != nil || got == nil {
		t.Fatalf("GetByID() = %v,%v", got, err)
	}
	if got.Description != "晚餐" {
		t.Errorf("description = %q, want 晚餐", got.Description)
	}
}

func TestDeleteRecord(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	r := &models.Record{Date: "2024-01-10", Amount: -50, Category: "餐饮", Description: "午餐"}
	if err := db.Create(ctx, r, 1); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if err := db.Delete(ctx, r.ID, 1); err != nil {
		t.Fatalf("Delete() = %v", err)
	}
	got, err := db.GetByID(ctx, r.ID, 1)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}
	if got != nil {
		t.Error("GetByID() should return nil after delete")
	}
}

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	n, err := db.UserCount(ctx)
	if err != nil {
		t.Fatalf("UserCount() = %v", err)
	}
	if n != 0 {
		t.Errorf("UserCount() = %d, want 0", n)
	}

	u := &models.User{Username: "testuser", Role: models.RoleAdmin}
	if err := db.CreateUser(ctx, u, "hash123"); err != nil {
		t.Fatalf("CreateUser() = %v", err)
	}
	if u.ID == 0 {
		t.Fatal("CreateUser() did not set ID")
	}

	got, err := db.GetUserByUsername(ctx, "testuser")
	if err != nil || got == nil {
		t.Fatalf("GetUserByUsername() = %v,%v", got, err)
	}
	if got.Username != "testuser" {
		t.Errorf("username = %q, want testuser", got.Username)
	}

	n, err = db.UserCount(ctx)
	if err != nil {
		t.Fatalf("UserCount() after create = %v", err)
	}
	if n != 1 {
		t.Errorf("UserCount() = %d, want 1", n)
	}

	list, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(list) = %d, want 1", len(list))
	}
}

func TestDailySummary(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: 1000, Category: "工资"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: -200, Category: "购物"}, 1)

	s, err := db.DailySummary(ctx, "2024-01-01", 1)
	if err != nil {
		t.Fatalf("DailySummary() = %v", err)
	}
	if s.Income != 1000 || s.Expense != 200 || s.Balance != 800 {
		t.Errorf("summary = income=%f expense=%f balance=%f, want 1000/200/800", s.Income, s.Expense, s.Balance)
	}
	if s.Count != 2 {
		t.Errorf("count = %d, want 2", s.Count)
	}
}

func TestUpdateRecord_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.Update(ctx, 999, 1, &models.UpdateRecordRequest{})
	if err != sql.ErrNoRows {
		t.Errorf("Update(999) = %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteRecord_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.Delete(ctx, 999, 1)
	if err != sql.ErrNoRows {
		t.Errorf("Delete(999) = %v, want sql.ErrNoRows", err)
	}
}

func TestDailySummary_NoRecords(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, err := db.DailySummary(ctx, "2099-01-01", 1)
	if err != nil {
		t.Fatalf("DailySummary() = %v", err)
	}
	if s.Income != 0 || s.Expense != 0 || s.Count != 0 {
		t.Errorf("summary = income=%f expense=%f count=%d, want zeros", s.Income, s.Expense, s.Count)
	}
	if len(s.Records) != 0 {
		t.Errorf("records = %d, want 0", len(s.Records))
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u, err := db.GetUserByUsername(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetUserByUsername() = %v", err)
	}
	if u != nil {
		t.Error("GetUserByUsername() should return nil for unknown user")
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.UpdateUser(ctx, 999, "test", models.RoleUser)
	if err != sql.ErrNoRows {
		t.Errorf("UpdateUser(999) = %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	err := db.DeleteUser(ctx, 999)
	if err != sql.ErrNoRows {
		t.Errorf("DeleteUser(999) = %v, want sql.ErrNoRows", err)
	}
}

func TestOperationLog(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := db.LogOperation(ctx, 1, "admin", "test_action", "record", "1", "test", "127.0.0.1", "agent"); err != nil {
		t.Fatalf("LogOperation() = %v", err)
	}

	list, total, err := db.ListOperationLogs(ctx, 1, 10, nil, "")
	if err != nil {
		t.Fatalf("ListOperationLogs() = %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("list: total=%d len=%d, want 1", total, len(list))
	}
	if list[0].Action != "test_action" {
		t.Errorf("action = %q, want test_action", list[0].Action)
	}
}

func TestLoginLog(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := db.LogLogin(ctx, nil, "unknown", false, "127.0.0.1", "curl"); err != nil {
		t.Fatalf("LogLogin() = %v", err)
	}
}

func TestEnvironmentVariable(t *testing.T) {
	// ensure JWT_SECRET env check is not needed in tests
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-only-1234")
	val := os.Getenv("JWT_SECRET")
	if val == "" {
		t.Error("JWT_SECRET not set")
	}
}

func TestMonthlySummary(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-01-05", Amount: 5000, Category: "工资"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-01-10", Amount: -300, Category: "购物"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-01-20", Amount: -150, Category: "餐饮"}, 1)

	s, err := db.MonthlySummary(ctx, 2024, 1, 1)
	if err != nil {
		t.Fatalf("MonthlySummary() = %v", err)
	}
	if s.Income != 5000 || s.Expense != 450 || s.Count != 3 {
		t.Errorf("summary = income=%f expense=%f count=%d, want 5000/450/3", s.Income, s.Expense, s.Count)
	}
	if s.Balance != 5000-450 {
		t.Errorf("balance = %f, want %f", s.Balance, float64(5000-450))
	}
	if len(s.Breakdown) == 0 {
		t.Error("breakdown should not be empty")
	}
}

func TestYearlySummary(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-01-15", Amount: 10000, Category: "奖金"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-06-15", Amount: -2000, Category: "旅游"}, 1)

	s, err := db.YearlySummary(ctx, 2024, 1)
	if err != nil {
		t.Fatalf("YearlySummary() = %v", err)
	}
	if s.Income != 10000 || s.Expense != 2000 || s.Count != 2 {
		t.Errorf("summary = income=%f expense=%f count=%d, want 10000/2000/2", s.Income, s.Expense, s.Count)
	}
	if s.Balance != 8000 {
		t.Errorf("balance = %f, want 8000", s.Balance)
	}
	if len(s.Breakdown) == 0 {
		t.Error("breakdown should not be empty")
	}
}

func TestYearlySummary_NoData(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, err := db.YearlySummary(ctx, 2099, 1)
	if err != nil {
		t.Fatalf("YearlySummary() = %v", err)
	}
	if s.Income != 0 || s.Expense != 0 || s.Count != 0 {
		t.Errorf("summary = income=%f expense=%f count=%d, want zeros", s.Income, s.Expense, s.Count)
	}
}

func TestReport(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-03-01", Amount: 3000, Category: "工资"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-03-02", Amount: -500, Category: "餐饮"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-03-03", Amount: -200, Category: "交通"}, 1)

	r, err := db.Report(ctx, "2024-03-01", "2024-03-31", 1)
	if err != nil {
		t.Fatalf("Report() = %v", err)
	}
	if r.Income != 3000 || r.Expense != 700 || r.Balance != 2300 || r.Count != 3 {
		t.Errorf("report = income=%f expense=%f balance=%f count=%d, want 3000/700/2300/3", r.Income, r.Expense, r.Balance, r.Count)
	}
	if len(r.Daily) == 0 {
		t.Error("daily breakdown should not be empty")
	}
	if len(r.ByCategory) == 0 {
		t.Error("category breakdown should not be empty")
	}
}

func TestReport_NoData(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	r, err := db.Report(ctx, "2099-01-01", "2099-12-31", 1)
	if err != nil {
		t.Fatalf("Report() = %v", err)
	}
	if r.Income != 0 || r.Expense != 0 || r.Count != 0 {
		t.Errorf("report = income=%f expense=%f count=%d, want zeros", r.Income, r.Expense, r.Count)
	}
	if len(r.Daily) != 0 {
		t.Errorf("daily = %d, want 0", len(r.Daily))
	}
	if len(r.ByCategory) != 0 {
		t.Errorf("byCategory = %d, want 0", len(r.ByCategory))
	}
}

func TestMonthlySummary_BalanceCalculation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-06-01", Amount: 2000, Category: "收入"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-06-15", Amount: -800, Category: "支出"}, 1)

	s, err := db.MonthlySummary(ctx, 2024, 6, 1)
	if err != nil {
		t.Fatalf("MonthlySummary() = %v", err)
	}
	if s.Balance != s.Income-s.Expense {
		t.Errorf("balance %f != income %f - expense %f", s.Balance, s.Income, s.Expense)
	}
	if len(s.Breakdown) > 0 {
		item := s.Breakdown[0]
		if item.Balance != item.Income-item.Expense {
			t.Errorf("breakdown balance %f != income %f - expense %f", item.Balance, item.Income, item.Expense)
		}
	}
}

func TestRecordOps_ZeroUserID_ReturnsUnauthorized(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: 100}, 0); err != ErrUnauthorized {
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
}

func TestRecordOps_NegativeUserID_ReturnsUnauthorized(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: 100}, -1); err != ErrUnauthorized {
		t.Errorf("Create(-1) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.GetByID(ctx, 1, -1); err != ErrUnauthorized {
		t.Errorf("GetByID(-1) = %v, want ErrUnauthorized", err)
	}
}

func TestSummaryOps_ZeroUserID_ReturnsUnauthorized(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := db.DailySummary(ctx, "2024-01-01", 0); err != ErrUnauthorized {
		t.Errorf("DailySummary(0) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.MonthlySummary(ctx, 2024, 1, 0); err != ErrUnauthorized {
		t.Errorf("MonthlySummary(0) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.YearlySummary(ctx, 2024, 0); err != ErrUnauthorized {
		t.Errorf("YearlySummary(0) = %v, want ErrUnauthorized", err)
	}
	if _, err := db.Report(ctx, "2024-01-01", "2024-01-31", 0); err != ErrUnauthorized {
		t.Errorf("Report(0) = %v, want ErrUnauthorized", err)
	}
}
