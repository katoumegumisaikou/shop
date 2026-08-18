package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/response"
)

const idempotencyTTL = 1 * time.Hour

// Idempotency 幂等中间件，基于 Idempotency-Key 请求头。
// 相同 key 在 TTL 内再次请求时：
//   - 若上次请求业务处理完成，Redis 中已写入响应体，则原样重放上一次响应。
//   - 若上次请求业务处理中，Redis 中尚无响应体，返回 409 提示请求正在处理。
//   - Redis 故障 / 缓存值损坏时降级返回 503 或 500，由调用方决定是否重试。
func Idempotency(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		key := c.GetHeader("Idempotency-Key")
		// 空 key 直接放行，避免所有未传 key 的请求共用同一个 redis key 导致全员被错杀。
		if key == "" {
			c.Next()
			return
		}
		redisKey := fmt.Sprintf("shop:idempotency:%d:%s", userID, key)
		set, err := rdb.SetNX(c.Request.Context(), redisKey, "", idempotencyTTL).Result()
		if err != nil {
			// redis故障，放行
			c.Next()
			return
		}

		if !set {
			// 设置失败，说明值已经存在，返回订单号和订单ID
			orderBytes, err := rdb.Get(c.Request.Context(), redisKey).Bytes()
			if err != nil {
				response.Error(c, errs.ErrServiceDegraded.WithMsg("服务繁忙,请稍后重试"))
				return
			}

			// 说明有请求进入业务层，但未完成
			if len(orderBytes) == 0 {
				response.Error(c, errs.ErrConflict.WithMsg("该 Idempotency-Key 正在处理中,请通过订单列表查询进度"))
				return
			}

			// 有结果，原样重放上一次成功响应（不关心业务类型，直接透传原始 JSON 字节）。
			if json.Valid(orderBytes) {
				response.OK(c, json.RawMessage(orderBytes))
				return
			}

			// 缓存值损坏：data integrity 问题，需要换 key 重试或联系客服
			response.Error(c, errs.ErrInternal.WithMsg("幂等缓存异常,请更换 Idempotency-Key 后重试或联系客服"))
			return
		}

		c.Next()
	}
}
