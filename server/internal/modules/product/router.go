package product

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	// 可选认证：登录则注入 user_id，未登录直接放行（详情页两者都支持）
	userOptionAuth := middleware.UserOptionalAuth(rdb, db, jwtCfg)
	userAuth := middleware.UserAuth(rdb, db, jwtCfg)
	// C 端
	c := r.Group("/c")
	{
		// 公开
		c.GET("/categories", middleware.PublicCache(300, 0), middleware.ETagMiddleware(), h.ListCategories)
		c.GET("/products", middleware.PublicCache(60, 120), middleware.ETagMiddleware(), h.ListProducts)
		c.GET("/products/:id", middleware.PublicCache(60, 120), middleware.ETagMiddleware(), userOptionAuth, h.GetProduct)

		c.POST("/favorites/:product_id", userAuth, h.AddFavorite)
		c.DELETE("/favorites/:product_id", userAuth, h.RemoveFavorite)
		c.GET("/favorites", userAuth, h.ListFavorites)
		c.GET("/view-history", userAuth, h.GetViewHistory)
	}

	// 后台端（预留：暂无路由，后续补充商品管理接口）
	r.Group("/admin")
}
