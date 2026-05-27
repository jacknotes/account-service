package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"account-service/internal/database"
	"account-service/internal/models"

	"github.com/gin-gonic/gin"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("database.New() = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupAuthContext(userID int64, username, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("username", username)
		c.Set("role", role)
		c.Next()
	}
}

func TestRecordHandler_CreateRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewRecordHandler(db, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/records", strings.NewReader(`{"date":"2024-01-15","amount":-50,"category":"餐饮","description":"午餐"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	setupAuthContext(1, "testuser", "admin")(c)

	// manually set gin context params
	h.CreateRecord(c)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"date":"2024-01-15"`) {
		t.Errorf("body = %s, should contain date", body)
	}

	// verify record exists in DB
	ctx := context.Background()
	list, total, err := db.List(ctx, &models.QueryParams{Page: 1, PageSize: 10}, 1)
	if err != nil {
		t.Fatalf("List() = %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("total=%d len=%d, want 1", total, len(list))
	}
}

func TestRecordHandler_ListRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewRecordHandler(db, db)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: 100, Category: "工资"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-01-02", Amount: -30, Category: "餐饮"}, 1)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/records", nil)
	setupAuthContext(1, "testuser", "admin")(c)

	h.ListRecords(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"total":2`) {
		t.Errorf("body = %s, should contain total:2", body)
	}
}

func TestRecordHandler_GetRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewRecordHandler(db, db)
	ctx := context.Background()

	r := &models.Record{Date: "2024-01-01", Amount: 100, Category: "工资"}
	db.Create(ctx, r, 1)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/records/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setupAuthContext(1, "testuser", "admin")(c)

	h.GetRecord(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecordHandler_GetRecord_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewRecordHandler(db, db)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/records/999", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setupAuthContext(1, "testuser", "admin")(c)

	h.GetRecord(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSummaryHandler_DailySummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewSummaryHandler(db)
	ctx := context.Background()

	db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: 1000, Category: "工资"}, 1)
	db.Create(ctx, &models.Record{Date: "2024-01-01", Amount: -200, Category: "购物"}, 1)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/summary/daily?date=2024-01-01", nil)
	setupAuthContext(1, "testuser", "admin")(c)

	h.DailySummary(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"income":1000`) || !strings.Contains(body, `"expense":200`) {
		t.Errorf("body = %s, should contain income:1000 and expense:200", body)
	}
}

func TestAuthHandler_RegisterStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewAuthHandler(db, "test-secret")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/auth/register/status", nil)

	h.RegisterStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"allow_register":true`) {
		t.Errorf("body = %s, should allow register", w.Body.String())
	}
}

func TestAuthHandler_RegisterAndLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	h := NewAuthHandler(db, "test-secret-key-for-testing-123456789")

	// Register
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	if w.Code != http.StatusCreated {
		t.Errorf("register status = %d, want 201. body: %s", w.Code, w.Body.String())
	}

	// Login
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	c2.Request.Header.Set("Content-Type", "application/json")

	h.Login(c2)

	if w2.Code != http.StatusOK {
		t.Errorf("login status = %d, want 200. body: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), `"token"`) {
		t.Errorf("login response should contain token: %s", w2.Body.String())
	}
}

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only!!!!")
	code := m.Run()
	os.Unsetenv("JWT_SECRET")
	os.Exit(code)
}
