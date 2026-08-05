package product

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	// C 端
	c := r.Group("/c")
	{
		// 公开
		c.GET("/categories", middleware.PublicCache(300, 0), middleware.ETagMiddleware(), h.ListCategories)
		c.GET("/products", middleware.PublicCache(60, 120), middleware.ETagMiddleware(), h.ListProducts)
	}
}
