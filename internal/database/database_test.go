package database

import (
	"context"
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
	list, total, err = db.List(ctx, params, 0)
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
