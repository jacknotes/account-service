package handlers

import (
	"testing"
	"time"
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
