package address

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	userAuth := middleware.UserAuth(rdb, db, jwtCfg)

	address := r.Group("/address", userAuth)
	{
		address.GET("/:id", h.Get)
		address.GET("", h.List)
		address.POST("/:id", h.Create)
		address.DELETE("/:id", h.Delete)
		address.PUT("", h.Update)
		address.POST("/default", h.SetDefault)
		address.POST("/decrypt-wx", h.DecryptWxAddress)
	}

	// 行政区划公开查询:挂在 r 上而不是 address group,避开 userAuth 中间件。
	// PublicCache 缓存 1 天(region 数据基本静态),ETag 用于 304 协商缓存。
	r.GET("/address/open/region",
		middleware.PublicCache(86400, 0),
		middleware.ETagMiddleware(),
		h.Region,
	)
}
