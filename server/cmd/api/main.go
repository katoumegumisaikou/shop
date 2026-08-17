package main

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"shop/internal/middleware"
	"shop/internal/modules/account"
	"shop/internal/modules/cart"
	"shop/internal/modules/product"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/logger"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/wxlogin"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	appLogger, err := logger.New(getEnv("APP_ENV", "dev"), getEnv("LOG_LEVEL", "info"))
	if err != nil {
		panic(err)
	}
	defer logger.Sync(appLogger)

	appLogger.Info("starting application")

	// 1. 初始化雪花 ID 生成器
	snowflake.Init(1)

	// 2. 初始化 JWT 配置（C 端 + Admin 端两套）
	pkgjwt.InitJwtConfig(
		pkgjwt.JwtConfig{JwtSecret: getEnv("USER_JWT_SECRET", "dev-user-secret")},
		pkgjwt.JwtConfig{JwtSecret: getEnv("ADMIN_JWT_SECRET", "dev-admin-secret")},
	)

	ctx := context.Background()

	// 3. 连接 PostgreSQL
	dsn := getEnv("DATABASE_URL",
		"host=localhost user=shop password=shop dbname=shop port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		appLogger.Fatal("connect postgres", zap.Error(err))
	}

	// 4. 连接 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: getEnv("REDIS_ADDR", "localhost:6379"),
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		appLogger.Fatal("connect redis", zap.Error(err))
	}

	// 5. 微信登录客户端（小程序 + 公众号，dev 阶段无真实 key 也可启动）
	wxMP := wxlogin.NewClient(
		getEnv("WX_MP_APPID", ""),
		getEnv("WX_MP_SECRET", ""),
	)
	wxOA := wxlogin.NewClient(
		getEnv("WX_OA_APPID", ""),
		getEnv("WX_OA_SECRET", ""),
	)

	// 6. Account 模块：repo → service → handler
	userRepo := account.NewUserRepo(db)
	adminRepo := account.NewAdminRepo(db)
	roleRepo := account.NewRoleRepo(db)
	svc := account.NewService(userRepo, adminRepo, roleRepo, rdb, wxMP, wxOA, appLogger)

	userCfg := pkgjwt.GetUserConfig()
	adminCfg := pkgjwt.GetAdminConfig()
	isProd := os.Getenv("APP_ENV") == "prod"
	handler := account.NewHandler(svc, userCfg, adminCfg, isProd, nil)

	// Cart 模块：构造 productRepo（仅 cart 需要，product 模块本身暂不注册路由），
	// 然后 repo → service → handler。
	productRepo := product.NewProductRepo(db)
	cartRepo := cart.NewCartRepo(db)
	cartSvc := cart.NewService(cartRepo, productRepo, appLogger)
	cartHandler := cart.NewHandler(cartSvc)

	// 7. 注册路由
	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		appLogger.Error("panic recovered",
			zap.Any("panic", recovered),
			zap.ByteString("stack", debug.Stack()),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	r.Use(middleware.AccessLog(appLogger))

	// CORS 跨域配置（允许前端 dev server 访问）
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:10086", "http://127.0.0.1:10086"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(middleware.PrometheusMiddleware())

	// 注册自定义 validator：mobile（中国大陆手机号）
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
			return regexp.MustCompile(`^1\d{10}$`).MatchString(fl.Field().String())
		})
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 所有模块路由挂载到 /api/v1 下
	api := r.Group("/api/v1")
	account.RegisterRoutes(api, handler, rdb, db, userCfg)
	cart.RegisterRoutes(api, cartHandler, rdb, db, userCfg)

	// 8. 启动
	addr := getEnv("LISTEN_ADDR", ":8080")
	appLogger.Info("server starting", zap.String("addr", addr))
	if err := r.Run(addr); err != nil {
		appLogger.Fatal("server stopped unexpectedly", zap.Error(err))
	}
}
