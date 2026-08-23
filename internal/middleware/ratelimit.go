package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// bucket 令牌桶，单实例内存实现。
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

type RateLimiter struct {
	rate    float64
	burst   int
	buckets sync.Map
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRateLimiter 创建内存令牌桶限流器（rate 个/秒，burst 突发容量）。
// 注意：内存限流按实例独立；单实例部署足够，多实例需要外部限流网关或 Redis。
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

func (rl *RateLimiter) Stop() {
	rl.cancel()
}

func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		now := time.Now()
		val, _ := rl.buckets.LoadOrStore(ip, &bucket{
			tokens:     float64(rl.burst),
			lastRefill: now,
		})
		b, ok := val.(*bucket)
		if !ok {
			c.Next()
			return
		}

		b.mu.Lock()
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
