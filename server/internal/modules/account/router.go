package account

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/middleware"
	pkgjwt "shop/internal/pkg/jwt"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler, rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) {
	userAuth := middleware.UserAuth(rdb, db, pkgjwt.GetUserConfig())
	adminAuth := middleware.AdminAuth(rdb, db, pkgjwt.GetAdminConfig())

	sensitive := middleware.SetSensitive()

	// 添加ip限流和用户限流
	userLimiter := middleware.UserRateLimiter(rdb)
	ipLimiter := middleware.IPRateLimiter(rdb)

	r.Use(ipLimiter)

	// C 端路由
	c := r.Group("/c")
	{
		auth := c.Group("/auth")
		auth.POST("/mp/login", h.MpLogin)
		auth.POST("/sms/code", h.SendSmsCode)
		auth.GET("/h5/code", h.H5GetOAuthURL)
		auth.GET("/h5/callback", h.H5Callback)
		// sensitive 必须在 userAuth 之前，否则 auth 中间件读取 sensitive flag 时仍为 false
		auth.POST("/bind-phone", sensitive, userAuth, userLimiter, h.BindPhone)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/logout", sensitive, userAuth, userLimiter, h.Logout)
		auth.POST("/phone-register", h.RegisterByPhone)
		auth.POST("/reset-password", userAuth, h.ResetPassword)
		auth.POST("/phone-login", h.PhoneLogin)

		// GET /c/me 可选认证：已登录返回用户信息，未登录返回 null（200）
		optionalAuth := middleware.UserOptionalAuth(rdb, db, jwtCfg)
		c.GET("/me", optionalAuth, h.GetMe)

		me := c.Group("/me", userAuth)
		// 注销属敏感操作；将 sensitive 放在 group 已注入的 userAuth 之前需单独绑定路由
		me.POST("/me/deactivate", sensitive, userAuth, h.RequestDeactivate)
		cdUserAuth := middleware.CancelDeactivateUserAuth(rdb, db, pkgjwt.GetUserConfig())
		me.POST("/me/deactivate/cancel", sensitive, cdUserAuth, h.CancelDeactivate)
		me.GET("/balance", h.GetMyBalance)
		me.PUT("", h.UpdateMe)

	}

	// Admin 路由
	admin := c.Group("/admin")
	{
		authGroup := admin.Group("/auth")
		authGroup.POST("/captcha", h.AdminGetCaptcha)
		authGroup.POST("/login", h.AdminLogin)
		authGroup.POST("/loginout", sensitive, adminAuth(""))
	}

}
