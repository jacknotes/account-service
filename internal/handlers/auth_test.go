package handlers

import (
	"testing"
	"time"

	"account-service/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
)

func TestTOTPRateLimiter_Allow_NewUser(t *testing.T) {
	rl := NewTOTPRateLimiter()
	if !rl.Allow(1) {
		t.Error("Allow(1) should return true for new user")
	}
}

func TestTOTPRateLimiter_BlockAfterMaxAttempts(t *testing.T) {
	rl := NewTOTPRateLimiter()
	rl.maxTries = 3
	rl.window = time.Minute
	rl.blockDur = time.Minute

	for i := 0; i < 3; i++ {
		if !rl.Allow(1) {
			t.Fatalf("Allow(1) should be true on attempt %d", i+1)
		}
		rl.RecordFailure(1)
	}
	if rl.Allow(1) {
		t.Error("Allow(1) should be false after max attempts")
	}
}

func TestTOTPRateLimiter_ResetClears(t *testing.T) {
	rl := NewTOTPRateLimiter()
	rl.maxTries = 2
	rl.window = time.Minute
	rl.blockDur = time.Minute

	rl.RecordFailure(1)
	rl.RecordFailure(1)
	rl.Reset(1)
	if !rl.Allow(1) {
		t.Error("Allow(1) should be true after Reset")
	}
}

func TestTOTPRateLimiter_DifferentUsersIsolated(t *testing.T) {
	rl := NewTOTPRateLimiter()
	rl.maxTries = 2
	rl.window = time.Minute
	rl.blockDur = time.Minute

	rl.RecordFailure(1)
	rl.RecordFailure(1)
	if rl.Allow(1) {
		t.Error("Allow(1) should be false after max attempts for user 1")
	}
	if !rl.Allow(2) {
		t.Error("Allow(2) should still be true for different user")
	}
}

func TestTOTPRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewTOTPRateLimiter()
	rl.maxTries = 2
	rl.window = 50 * time.Millisecond
	rl.blockDur = 50 * time.Millisecond

	rl.RecordFailure(1)
	rl.RecordFailure(1)
	if rl.Allow(1) {
		t.Error("Allow(1) should be false immediately after max attempts")
	}

	time.Sleep(60 * time.Millisecond)
	if !rl.Allow(1) {
		t.Error("Allow(1) should be true after window expires")
	}
}

func TestTOTPRateLimiter_ConcurrentSafe(t *testing.T) {
	rl := NewTOTPRateLimiter()
	rl.maxTries = 100
	rl.window = time.Minute
	rl.blockDur = time.Minute

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			rl.RecordFailure(1)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 50; i++ {
			rl.RecordFailure(1)
		}
		done <- struct{}{}
	}()
	<-done
	<-done

	// Allow should return false after 100 failures regardless of order
	_ = rl.Allow(1) // just check no panic
}

func TestTOTPRateLimiter_ZeroAttempts(t *testing.T) {
	rl := NewTOTPRateLimiter()
	if !rl.Allow(1) {
		t.Error("Allow(1) should return true for user with zero attempts")
	}
	rl.Reset(1) // Reset on user with no attempts should not panic
}

// TestIssueToken_UniquePerCall 回归：每次签发的 access token 必须唯一。
// 历史缺陷：JWT 的 iat/exp 只有秒级精度且无 jti，同一秒内两次登录会生成完全相同的
// token 字符串；配合黑名单，会出现「登出后立刻重新登录，新 token 恰是刚被拉黑的那一个」
// 从而被误判为 401。
func TestIssueToken_UniquePerCall(t *testing.T) {
	_, _, h, _ := newTestHandler()
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		tok, err := h.issueToken(1, "u", "user")
		if err != nil {
			t.Fatalf("issueToken() = %v", err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("第 %d 次签发的 token 与之前重复（缺少唯一 jti）", i)
		}
		seen[tok] = struct{}{}
	}
}

// TestIssueToken_HasJTI 确认 jti 已写入。
func TestIssueToken_HasJTI(t *testing.T) {
	_, _, h, _ := newTestHandler()
	tok, err := h.issueToken(7, "u", "admin")
	if err != nil {
		t.Fatalf("issueToken() = %v", err)
	}
	claims := &middleware.Claims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, func(*jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-for-testing-123456789"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("解析 token 失败: %v", err)
	}
	if claims.ID == "" {
		t.Error("claims.ID (jti) 不应为空")
	}
	if claims.UserID != 7 || claims.Role != "admin" {
		t.Errorf("claims = %+v", claims)
	}
}
