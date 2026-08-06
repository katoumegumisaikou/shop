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
	// 后台权限认证：adminAuth("权限码") 返回带权限校验的中间件
	adminAuth := middleware.AdminAuth(rdb, db, jwtCfg)
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

	// 后台端：分类管理 + 商品管理
	admin := r.Group("/admin")
	{
		// 后台 - 分类管理
		admin.GET("/categories", adminAuth("category.view"), h.AdminListCategories)
		admin.POST("/categories", adminAuth("category.create"), h.AdminCreateCategory)
		admin.PUT("/categories/:id", adminAuth("category.edit"), h.AdminUpdateCategory)
		admin.DELETE("/categories/:id", adminAuth("category.delete"), h.AdminDeleteCategory)

		// 后台 - 商品管理
		admin.GET("/products", adminAuth("product.view"), h.AdminListProducts)
		admin.POST("/products", adminAuth("product.create"), h.AdminCreateProduct)
		admin.GET("/products/:id", adminAuth("product.view"), h.AdminGetProduct)
		admin.PUT("/products/:id", adminAuth("product.edit"), h.AdminUpdateProduct)
		admin.DELETE("/products/:id", adminAuth("product.delete"), h.AdminDeleteProduct)
		admin.POST("/products/:id/copy", adminAuth("product.create"), h.AdminCopyProduct)
		admin.POST("/products/:id/onsale", adminAuth("product.edit"), h.AdminOnSale)
		admin.POST("/products/:id/offsale", adminAuth("product.edit"), h.AdminOffSale)
		admin.POST("/products/batch-status", adminAuth("product.edit"), h.AdminBatchStatus)
		admin.POST("/skus/batch-price", adminAuth("product.edit"), h.AdminBatchPrice)
	}
}
