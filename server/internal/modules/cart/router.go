package cart

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/response"
)

// RegisterRoutes 注册购物车模块路由。
func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	userAuth := middleware.UserAuth(rdb, db, jwtCfg)

	// 内联限流：仅对 POST /c/cart (Add) 限速 60 次/分钟，key 含 userID。
	cartAddRL := func(c *gin.Context) {
		limiter := redis_rate.NewLimiter(rdb)
		userID := c.GetInt64("user_id")
		key := fmt.Sprintf("shop:rl:cart:add:%d", userID)
		if err := middleware.RateLimiter(c.Request.Context(), limiter, key, redis_rate.PerMinute(60)); err != nil {
			response.Error(c, err)
			return
		}
		c.Next()
	}

	g := r.Group("/c/cart", userAuth)
	{
		g.GET("", h.List)
		g.POST("", cartAddRL, h.Add)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
		g.POST("/batch-delete", h.BatchDelete)
		g.POST("/clean-invalid", h.CleanInvalid)
		g.GET("/count", h.Count)
		g.POST("/precheck", h.Precheck)
	}
}