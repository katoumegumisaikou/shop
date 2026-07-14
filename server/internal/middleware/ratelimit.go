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
	// ipPerMinuteRequest 每个 IP 每分钟允许的请求数，用于 IPRateLimiter 全局兜底限流。
	ipPerMinuteRequest = 10
	// userPerMinuteRequest 每个已登录用户每分钟允许的请求数，用于敏感操作（如绑定手机、登出）。
	userPerMinuteRequest = 10
	// captchaPerMinuteRequest 每个 IP 每分钟允许的验证码请求数，用于 CaptchaRateLimiter 限制 admin 登录验证码频率。
	captchaPerMinuteRequest = 3
)

func RateLimiter(ctx context.Context, limiter *redis_rate.Limiter, key string, limit redis_rate.Limit) error {
	// 这里redis宕机时放行请求
	result, _ := limiter.Allow(ctx, key, limit)
	if result.Allowed < 1 {
		msg := fmt.Sprintf("请 %d 秒后重新尝试", result.RetryAfter/time.Second)
		return errs.ErrRateLimit.WithMsg(msg)
	}
	return nil
}

func IPRateLimiter(client *redis.Client) gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(client)
	return func(c *gin.Context) {
		ipKey := fmt.Sprintf("shop:rl:api:%s:%s", c.FullPath(), c.ClientIP())
		err := RateLimiter(c.Request.Context(), limiter, ipKey, redis_rate.PerMinute(ipPerMinuteRequest))
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

func CaptchaRateLimiter(client *redis.Client) gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(client)
	return func(c *gin.Context) {
		captchaKey := fmt.Sprintf("shop:rl:captcha:ip:%s", c.ClientIP())
		err := RateLimiter(c.Request.Context(), limiter, captchaKey, redis_rate.PerMinute(captchaPerMinuteRequest))
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Next()
	}
}
