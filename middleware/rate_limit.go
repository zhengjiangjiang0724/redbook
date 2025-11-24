package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"redbook/internal/metrics"
)

var rlCtx = context.Background()

// LoginRateLimiter limits login attempts per client IP using Redis counters.
func LoginRateLimiter(rdb *redis.Client, limit int64, window time.Duration) gin.HandlerFunc {
	const limiterName = "login"
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rb:rl:%s:%s", limiterName, ip)

		count, err := rdb.Incr(rlCtx, key).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rate limiter failed"})
			return
		}
		if count == 1 {
			_ = rdb.Expire(rlCtx, key, window).Err()
		}
		if count > limit {
			metrics.IncRateLimit(limiterName)
			c.Header("Retry-After", fmt.Sprintf("%.f", window.Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
			return
		}
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count))
		c.Next()
	}
}
