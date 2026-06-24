package account

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	userAuth := middleware.UserAuth(rdb, db, jwtCfg)
	sensitive := middleware.SetSensitive()

	// C 端路由
	c := r.Group("/c")
	{
		auth := c.Group("/auth")
		auth.POST("/mp/login", h.MpLogin)
		auth.GET("/h5/code", h.H5GetOAuthURL)
		auth.GET("/h5/callback", h.H5Callback)
		// sensitive 必须在 userAuth 之前，否则 auth 中间件读取 sensitive flag 时仍为 false
		auth.POST("/bind-phone", sensitive, userAuth, h.BindPhone)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/logout", sensitive, userAuth, h.Logout)
	}
}
