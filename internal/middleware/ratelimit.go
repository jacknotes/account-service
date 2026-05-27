package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

// RateLimiter implements a token-bucket rate limiter keyed by client IP.
type RateLimiter struct {
	rate    float64
	burst   int
	buckets sync.Map
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRateLimiter creates a new RateLimiter.
// rate is the number of tokens replenished per second,
// burst is the maximum number of tokens a bucket can hold.
func NewRateLimiter(rate int, burst int) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		rate:   float64(rate),
		burst:  burst,
		ctx:    ctx,
		cancel: cancel,
	}
	go rl.cleanup()
	return rl
}

// Stop terminates the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.cancel()
}

// Limit returns a Gin middleware handler that enforces per-IP rate limiting.
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		now := time.Now()
		val, _ := rl.buckets.LoadOrStore(ip, &bucket{
			tokens:    float64(rl.burst),
			lastRefill: now,
		})
		b, ok := val.(*bucket)
		if !ok {
			c.Next()
			return
		}

		b.mu.Lock()
		// Refill tokens based on elapsed time
		elapsed := now.Sub(b.lastRefill).Seconds()
		b.tokens += elapsed * rl.rate
		if b.tokens > float64(rl.burst) {
			b.tokens = float64(rl.burst)
		}
		b.lastRefill = now

		if b.tokens < 1 {
			b.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}

		b.tokens--
		b.mu.Unlock()
		c.Next()
	}
}

// cleanup periodically removes entries that have not been accessed for more than 1 minute.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-1 * time.Minute)
			rl.buckets.Range(func(key, value interface{}) bool {
				b, ok := value.(*bucket)
				if !ok {
					return true
				}
				b.mu.Lock()
				shouldDelete := b.lastRefill.Before(cutoff)
				b.mu.Unlock()
				if shouldDelete {
					rl.buckets.Delete(key)
				}
				return true
			})
		}
	}
}
