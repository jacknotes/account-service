package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"account-service/internal/models"
	"account-service/internal/service"

	"github.com/gin-gonic/gin"
)

// ----------------------------------------------------------------------
// 内存 fakes（不再依赖真实数据库，便于 `go test ./...` 无外部服务即可运行）
// ----------------------------------------------------------------------

type fakeRecordService struct {
	records  map[int64]*models.Record
	nextID   int64
	lastList *models.QueryParams
}

func newFakeRecordService() *fakeRecordService {
	return &fakeRecordService{records: make(map[int64]*models.Record), nextID: 1}
}

func (f *fakeRecordService) Create(_ context.Context, r *models.Record, userID int64) error {
	r.ID = f.nextID
	r.UserID = userID
	f.records[r.ID] = r
	f.nextID++
	return nil
}

func (f *fakeRecordService) GetByID(_ context.Context, id, _ int64) (*models.Record, error) {
	r, ok := f.records[id]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (f *fakeRecordService) List(_ context.Context, params *models.QueryParams, _ int64) ([]*models.Record, int64, error) {
	f.lastList = params
	var list []*models.Record
	for _, r := range f.records {
		list = append(list, r)
	}
	return list, int64(len(list)), nil
}

func (f *fakeRecordService) Update(_ context.Context, id, _ int64, req *models.UpdateRecordRequest) error {
	r, ok := f.records[id]
	if !ok {
		return sql.ErrNoRows
	}
	if req.Date != nil {
		r.Date = *req.Date
	}
	if req.AmountCents != nil {
		r.AmountCents = *req.AmountCents
	}
	if req.Category != nil {
		r.Category = *req.Category
	}
	if req.Description != nil {
		r.Description = *req.Description
	}
	return nil
}

func (f *fakeRecordService) Delete(_ context.Context, id, _ int64) error {
	if _, ok := f.records[id]; !ok {
		return sql.ErrNoRows
	}
	delete(f.records, id)
	return nil
}

type fakeSummaryService struct {
	daily   *models.Summary
	monthly *models.Summary
	yearly  *models.Summary
	report  *models.Report
}

func (f *fakeSummaryService) DailySummary(_ context.Context, _ string, _ int64) (*models.Summary, error) {
	if f.daily != nil {
		return f.daily, nil
	}
	return &models.Summary{}, nil
}
func (f *fakeSummaryService) MonthlySummary(_ context.Context, _, _ int, _ int64) (*models.Summary, error) {
	if f.monthly != nil {
		return f.monthly, nil
	}
	return &models.Summary{}, nil
}
func (f *fakeSummaryService) YearlySummary(_ context.Context, _ int, _ int64) (*models.Summary, error) {
	if f.yearly != nil {
		return f.yearly, nil
	}
	return &models.Summary{}, nil
}
func (f *fakeSummaryService) Report(_ context.Context, _, _ string, _ int64) (*models.Report, error) {
	if f.report != nil {
		return f.report, nil
	}
	return &models.Report{}, nil
}

type fakeUserService struct {
	byName    map[string]*models.User
	byID      map[int64]*models.User
	nextID    int64
	refresh   map[string]int64 // token_hash -> userID
	blacklist map[string]bool
}

func newFakeUserService() *fakeUserService {
	return &fakeUserService{
		byName:    make(map[string]*models.User),
		byID:      make(map[int64]*models.User),
		nextID:    1,
		refresh:   make(map[string]int64),
		blacklist: make(map[string]bool),
	}
}

func (f *fakeUserService) CreateUser(_ context.Context, u *models.User, passwordHash string) error {
	u.ID = f.nextID
	u.PasswordHash = passwordHash
	f.byName[u.Username] = u
	f.byID[u.ID] = u
	f.nextID++
	return nil
}

func (f *fakeUserService) CreateFirstUser(_ context.Context, u *models.User, passwordHash string) error {
	if len(f.byID) > 0 {
		return sql.ErrNoRows // 表示“已存在用户”，上层转为“注册已关闭”
	}
	u.ID = f.nextID
	u.PasswordHash = passwordHash
	f.byName[u.Username] = u
	f.byID[u.ID] = u
	f.nextID++
	return nil
}

func (f *fakeUserService) GetUserByID(_ context.Context, id int64) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (f *fakeUserService) GetUserByUsername(_ context.Context, username string) (*models.User, error) {
	if u, ok := f.byName[username]; ok {
		return u, nil
	}
	return nil, nil
}

func (f *fakeUserService) UpdateUserPassword(_ context.Context, id int64, passwordHash string) error {
	if u, ok := f.byID[id]; ok {
		u.PasswordHash = passwordHash
		return nil
	}
	return sql.ErrNoRows
}

func (f *fakeUserService) SetTOTPSecret(_ context.Context, id int64, secret string) error {
	if u, ok := f.byID[id]; ok {
		u.TOTPSecret = secret
		return nil
	}
	return sql.ErrNoRows
}

func (f *fakeUserService) UserCount(_ context.Context) (int, error) {
	return len(f.byID), nil
}

func (f *fakeUserService) ListUsers(_ context.Context) ([]*models.User, error) {
	var list []*models.User
	for _, u := range f.byID {
		list = append(list, u)
	}
	return list, nil
}

func (f *fakeUserService) UpdateUser(_ context.Context, id int64, username, role string) error {
	u, ok := f.byID[id]
	if !ok {
		return sql.ErrNoRows
	}
	delete(f.byName, u.Username)
	u.Username = username
	u.Role = role
	f.byName[username] = u
	return nil
}

func (f *fakeUserService) DeleteUser(_ context.Context, id int64) error {
	u, ok := f.byID[id]
	if !ok {
		return sql.ErrNoRows
	}
	delete(f.byID, id)
	delete(f.byName, u.Username)
	return nil
}

func (f *fakeUserService) SaveRefreshToken(_ context.Context, userID int64, tokenHash string, _ time.Time) error {
	f.refresh[tokenHash] = userID
	return nil
}

func (f *fakeUserService) GetRefreshToken(_ context.Context, tokenHash string) (int64, error) {
	uid, ok := f.refresh[tokenHash]
	if !ok {
		return 0, nil
	}
	return uid, nil
}

func (f *fakeUserService) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	delete(f.refresh, tokenHash)
	return nil
}

func (f *fakeUserService) RevokeAllRefreshTokensForUser(_ context.Context, userID int64) error {
	for h, uid := range f.refresh {
		if uid == userID {
			delete(f.refresh, h)
		}
	}
	return nil
}

func (f *fakeUserService) BlacklistToken(_ context.Context, tokenHash string, _ time.Time) error {
	f.blacklist[tokenHash] = true
	return nil
}

func (f *fakeUserService) IsTokenBlacklisted(_ context.Context, tokenHash string) (bool, error) {
	return f.blacklist[tokenHash], nil
}

type fakeOpLogService struct{}

func (f *fakeOpLogService) LogOperation(_ context.Context, _ int64, _ string, _ string, _ string, _ string, _ string, _ string, _ string) error {
	return nil
}
func (f *fakeOpLogService) LogLogin(_ context.Context, _ *int64, _ string, _ bool, _ string, _ string) error {
	return nil
}
func (f *fakeOpLogService) ListOperationLogs(_ context.Context, _ int, _ int, _ *int64, _ string) ([]*service.OperationLogEntry, int64, error) {
	return nil, 0, nil
}

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

// ----------------------------------------------------------------------
// 测试辅助
// ----------------------------------------------------------------------

func setupAuthContext(userID int64, username, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("username", username)
		c.Set("role", role)
		c.Next()
	}
}

func newTestHandler() (*RecordHandler, *SummaryHandler, *AuthHandler, *fakeUserService) {
	records := newFakeRecordService()
	users := newFakeUserService()
	auth := NewAuthHandler(users, &fakeOpLogService{}, "test-secret-key-for-testing-123456789")
	return NewRecordHandler(records, &fakeOpLogService{}), NewSummaryHandler(&fakeSummaryService{}), auth, users
}

func perform(h func(*gin.Context), method, target, body string, setup gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if setup != nil {
		setup(c)
	}
	h(c)
	return w
}

// ----------------------------------------------------------------------
// record handler
// ----------------------------------------------------------------------

func TestRecordHandler_CreateRecord(t *testing.T) {
	rh, _, _, _ := newTestHandler()
	w := perform(rh.CreateRecord, "POST", "/api/records", `{"date":"2024-01-15","amount_cents":-5000,"category":"餐饮","description":"午餐"}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"amount_cents":-5000`) {
		t.Errorf("body should contain amount_cents: %s", w.Body.String())
	}
}

func TestRecordHandler_CreateRecord_InvalidDate(t *testing.T) {
	rh, _, _, _ := newTestHandler()
	w := perform(rh.CreateRecord, "POST", "/api/records", `{"date":"2024-13-99","amount_cents":100}`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestRecordHandler_CreateRecord_InvalidJSON(t *testing.T) {
	rh, _, _, _ := newTestHandler()
	w := perform(rh.CreateRecord, "POST", "/api/records", `{invalid`, setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestRecordHandler_ListRecords_SortParams(t *testing.T) {
	rh, _, _, _ := newTestHandler()
	w := perform(rh.ListRecords, "GET", "/api/records?sort_field=amount&sort_dir=asc", "", setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRecordHandler_GetRecord_NotFound(t *testing.T) {
	rh, _, _, _ := newTestHandler()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/records/999", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setupAuthContext(1, "u", "user")(c)
	rh.GetRecord(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRecordHandler_Update_Delete_NotFound(t *testing.T) {
	rh, _, _, _ := newTestHandler()

	// Update（需手动设置 :id 路由参数，perform() 助手不解析路径参数）
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/records/999", strings.NewReader(`{"date":"2024-01-01"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setupAuthContext(1, "u", "user")(c)
	rh.UpdateRecord(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("update status = %d, want 404, body: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/records/999", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setupAuthContext(1, "u", "user")(c)
	rh.DeleteRecord(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete status = %d, want 404", w.Code)
	}
}

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

// ----------------------------------------------------------------------
// summary handler
// ----------------------------------------------------------------------

func TestSummaryHandler_DailySummary(t *testing.T) {
	_, sh, _, _ := newTestHandler()
	w := perform(sh.DailySummary, "GET", "/api/summary/daily?date=2024-01-01", "", setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestSummaryHandler_DailySummary_BadDate(t *testing.T) {
	_, sh, _, _ := newTestHandler()
	w := perform(sh.DailySummary, "GET", "/api/summary/daily?date=not-a-date", "", setupAuthContext(1, "u", "user"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ----------------------------------------------------------------------
// auth handler
// ----------------------------------------------------------------------

func TestAuthHandler_Register_ShortUsername(t *testing.T) {
	_, _, ah, _ := newTestHandler()
	w := perform(ah.Register, "POST", "/api/auth/register", `{"username":"a","password":"Admin@123"}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuthHandler_RegisterAndLogin(t *testing.T) {
	_, _, ah, _ := newTestHandler()

	w := perform(ah.Register, "POST", "/api/auth/register", `{"username":"admin","password":"Admin@123"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %s", w.Code, w.Body.String())
	}

	w = perform(ah.Login, "POST", "/api/auth/login", `{"username":"admin","password":"Admin@123"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token"`) || !strings.Contains(w.Body.String(), `"refresh_token"`) {
		t.Errorf("login response missing token/refresh_token: %s", w.Body.String())
	}
}

func TestAuthHandler_Refresh_RotatesToken(t *testing.T) {
	_, _, ah, users := newTestHandler()
	// 仅注册（签发一个 refresh token），避免登录再签发一个干扰计数
	w := perform(ah.Register, "POST", "/api/auth/register", `{"username":"admin","password":"Admin@123"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %s", w.Code, w.Body.String())
	}
	rt := extractJSONField(w.Body.String(), "refresh_token")
	if rt == "" {
		t.Fatal("register response missing refresh_token")
	}

	w = perform(ah.Refresh, "POST", "/api/auth/refresh", `{"refresh_token":"`+rt+`"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200: %s", w.Code, w.Body.String())
	}
	// 旧 token 应已被轮换撤销，只剩一个新 token
	if len(users.refresh) != 1 {
		t.Fatalf("after rotation should have exactly 1 live refresh token, got %d", len(users.refresh))
	}
	// 用旧 token 再次刷新应失败
	w2 := perform(ah.Refresh, "POST", "/api/auth/refresh", `{"refresh_token":"`+rt+`"}`, nil)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("replay of old refresh token status = %d, want 401", w2.Code)
	}
}

func TestAuthHandler_Logout_RevokesTokens(t *testing.T) {
	_, _, ah, users := newTestHandler()
	// 仅注册（签发一个 refresh token），避免登录再签发一个干扰计数
	w := perform(ah.Register, "POST", "/api/auth/register", `{"username":"admin","password":"Admin@123"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %s", w.Code, w.Body.String())
	}
	rt := extractJSONField(w.Body.String(), "refresh_token")
	if rt == "" {
		t.Fatal("register response missing refresh_token")
	}

	w = perform(ah.Logout, "POST", "/api/auth/logout", `{"refresh_token":"`+rt+`"}`, setupAuthContext(1, "admin", "admin"))
	if w.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if len(users.refresh) != 0 {
		t.Errorf("refresh tokens should be revoked after logout, got %d", len(users.refresh))
	}
}

// extractJSONField 从 JSON 字符串中提取指定字段的字符串值（测试用，非完整 JSON 解析）。
func extractJSONField(body, field string) string {
	needle := `"` + field + `":"`
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(needle):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// ----------------------------------------------------------------------
// 纯函数单元测试
// ----------------------------------------------------------------------

func TestValidatePasswordStrength(t *testing.T) {
	if err := validatePasswordStrength("Abcdef1!"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	if err := validatePasswordStrength("short1!"); err == nil {
		t.Error("too-short password accepted")
	}
	if err := validatePasswordStrength("abcdefgh"); err == nil {
		t.Error("no special/digit password accepted")
	}
	// 超长密码（>72 字节）应被拒绝，避免 bcrypt 静默截断
	long := strings.Repeat("Ab1!", 30) // 120 字节
	if err := validatePasswordStrength(long); err == nil {
		t.Error("over-72-byte password accepted")
	}
}

func TestIsValidDate(t *testing.T) {
	for _, good := range []string{"2024-01-15", "2024-12-31", "2000-02-29"} {
		if !isValidDate(good) {
			t.Errorf("isValidDate(%q) = false, want true", good)
		}
	}
	for _, bad := range []string{"2024-13-01", "2024-02-30", "20240101", "abc", ""} {
		if isValidDate(bad) {
			t.Errorf("isValidDate(%q) = true, want false", bad)
		}
	}
}
