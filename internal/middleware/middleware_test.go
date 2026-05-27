package middleware

import (
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

	Auth(secret)(c)
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

	Auth(secret)(c)
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

	Auth(secret)(c)
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

	Auth(secret)(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
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
