package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func testToken(t *testing.T, secret string, claims *Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() = %v", err)
	}
	return s
}

func setupAuthContext(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("username", "testuser")
		c.Set("role", role)
		c.Next()
	}
}

func TestAuth_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-for-testing-12345678"
	token := testToken(t, secret, &Claims{
		UserID:   42,
		Username: "alice",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	Auth(secret, nil)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if uid, _ := c.Get("user_id"); uid != int64(42) {
		t.Errorf("user_id = %v, want 42", uid)
	}
	if username, _ := c.Get("username"); username != "alice" {
		t.Errorf("username = %v, want alice", username)
	}
	if role, _ := c.Get("role"); role != "admin" {
		t.Errorf("role = %v, want admin", role)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-for-testing-12345678"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	Auth(secret, nil)(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-for-testing-12345678"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer invalidtoken")

	Auth(secret, nil)(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-key-for-testing-12345678"
	token := testToken(t, secret, &Claims{
		UserID:   1,
		Username: "test",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	Auth(secret, nil)(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// fakeBlacklister 可控返回值的黑名单实现。
type fakeBlacklister struct {
	listed bool
	err    error
}

func (f *fakeBlacklister) IsTokenBlacklisted(_ context.Context, _ string) (bool, error) {
	return f.listed, f.err
}

func validTestToken(t *testing.T, secret string) string {
	t.Helper()
	return testToken(t, secret, &Claims{
		UserID:   1,
		Username: "test",
		Role:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
}

func doAuth(secret, token string, bl TokenBlacklister) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	Auth(secret, bl)(c)
	return w
}

// 已拉黑的 token 必须被拒绝（登出/改密后不可继续使用）。
func TestAuth_BlacklistedTokenRejected(t *testing.T) {
	const secret = "test-secret-key-for-testing-12345678"
	w := doAuth(secret, validTestToken(t, secret), &fakeBlacklister{listed: true})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// 未拉黑的 token 正常放行。
func TestAuth_NotBlacklistedAllowed(t *testing.T) {
	const secret = "test-secret-key-for-testing-12345678"
	w := doAuth(secret, validTestToken(t, secret), &fakeBlacklister{listed: false})
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// fail-closed 回归：黑名单查询出错时必须拒绝，而不是放行。
// 历史缺陷正是「查询恒不命中/出错即放行」，使登出后的 token 仍可用。
func TestAuth_BlacklistErrorFailClosed(t *testing.T) {
	const secret = "test-secret-key-for-testing-12345678"
	w := doAuth(secret, validTestToken(t, secret), &fakeBlacklister{err: errors.New("db down")})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (fail-closed)", w.Code)
	}
}

func TestRequireAdmin_AdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	setupAuthContext("admin")(c)

	RequireAdmin()(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRequireAdmin_UserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	setupAuthContext("user")(c)

	RequireAdmin()(c)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestRateLimiter_FirstRequestPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(100, 100)
	defer rl.Stop()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Forwarded-For", "10.0.0.1")

	rl.Limit()(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	// Consume the only token for IP A
	wA := httptest.NewRecorder()
	cA, _ := gin.CreateTestContext(wA)
	cA.Request = httptest.NewRequest("GET", "/", nil)
	cA.Request.Header.Set("X-Forwarded-For", "10.0.0.1")
	rl.Limit()(cA)

	// IP B should still work
	wB := httptest.NewRecorder()
	cB, _ := gin.CreateTestContext(wB)
	cB.Request = httptest.NewRequest("GET", "/", nil)
	cB.Request.Header.Set("X-Forwarded-For", "10.0.0.2")
	rl.Limit()(cB)
	if wB.Code != http.StatusOK {
		t.Errorf("IP B status = %d, want 200", wB.Code)
	}
}
