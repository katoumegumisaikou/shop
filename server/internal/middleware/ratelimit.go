package middleware

import (
	"context"
	"fmt"
	"shop/internal/pkg/errs"
	"shop/internal/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

const (
	//
	apiPerMinuteRequest  = 10
	userPerMinuteRequest = 10
)

func RateLimiter(ctx context.Context, limiter *redis_rate.Limiter, key string, limit redis_rate.Limit) error {
	result, err := limiter.Allow(ctx, key, limit)
	if err != nil {
		return errs.ErrServiceDegraded
	} else if result.Allowed == 0 {
		msg := fmt.Sprintf("请 %d 秒后重新尝试", result.RetryAfter/time.Second)
		return errs.ErrRateLimit.WithMsg(msg)
	}
	return nil
}

func APIRateLimiter(client *redis.Client) gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(client)
	return func(c *gin.Context) {
		ipKey := fmt.Sprintf("shop:rl:api:%s:%s", c.FullPath(), c.ClientIP())
		err := RateLimiter(c.Request.Context(), limiter, ipKey, redis_rate.PerMinute(apiPerMinuteRequest))
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Next()

	}
}

func UserRateLimiter(client *redis.Client) gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(client)
	return func(c *gin.Context) {
		userID := c.GetInt64(ctxKeyUserID)
		if userID == 0 {
			response.Error(c, errs.ErrInternal)
			return
		}
		userKey := fmt.Sprintf("shop:rl:user:%s:%d", c.FullPath(), userID)
		err := RateLimiter(c.Request.Context(), limiter, userKey, redis_rate.PerMinute(userPerMinuteRequest))
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Next()

	}
}
