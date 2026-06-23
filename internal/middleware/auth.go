package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/response"
)

const (
	ctxKeySensitive = "sensitive"
	// userStatusCacheTTL user 状态 Redis 缓存时长。
	userStatusCacheTTL = 60 * time.Second
	// adminStatusCacheTTL admin 状态 Redis 缓存时长
	adminStatusCacheTTL = 60 * time.Second

	ctxKeyUserID  = "user_id"
	ctxKeyAdminID = "admin_id"
)

func UserAuth(rdb *redis.Client, db *gorm.DB, jwtCfg pkgjwt.JwtConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := getJWTTokenFromCtx(c)
		if !ok {
			response.Error(c, errs.ErrUnauth)
			return
		}

		claims, err := pkgjwt.Parse(jwtCfg.JwtSecret, tokenStr)
		if err != nil {
			response.Error(c, errs.ErrUnauth)
			return
		}

		// 查看这个接口操作是否是敏感操作
		sensitive := c.GetBool(ctxKeySensitive)

		// 查看是否在redis黑名单中
		isExist, err := isBlacklisted(c.Request.Context(), rdb, claims.JTI)
		if err != nil {
			if sensitive {
				// 敏感
				response.Error(c, errs.ErrUnauth)
				return
			}
		}
		if isExist {
			// 存在黑名单中
			response.Error(c, errs.ErrUnauth)
			return
		}

		// 查看用户是否被禁止
		status, err := getUserStatus(c.Request.Context(), rdb, db, claims.Sub)
		if err != nil {
			response.Error(c, errs.ErrInternal)
			return
		} else if status != "active" {
			response.Error(c, errs.ErrAccountLocked)
			return
		}

		c.Set(ctxKeyUserID, claims.Sub)
		c.Next()

	}
}

// getJWTTokenFromCtx 将jwtToken从gin.Context中取出，可能存在http头，也有可能存在cookie中
func getJWTTokenFromCtx(c *gin.Context) (string, bool) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader != "" {
		fields := strings.Fields(authHeader)
		if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") && fields[1] != "" {
			return fields[1], true
		}
		return "", false
	}

	token, err := c.Cookie("access_token")
	if err != nil {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

// isBlacklisted 检查 JTI 是否在 Redis 黑名单中。
func isBlacklisted(ctx context.Context, rdb *redis.Client, jti string) (bool, error) {
	result, err := rdb.Exists(ctx, fmt.Sprintf("jwt:bl:%s", jti)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func getUserStatus(ctx context.Context, rdb *redis.Client, db *gorm.DB, userID int64) (string, error) {
	key := fmt.Sprintf("user:status:%d", userID)
	status, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			return "", err
		}
	} else {
		return status, nil
	}

	err = db.WithContext(ctx).
		Table(`"user"`).
		Select("status").
		Where("id = ?", userID).
		Scan(&status).Error
	if err != nil {
		return "", err
	}
	_ = rdb.Set(ctx, key, status, userStatusCacheTTL)

	return status, nil
}

func SetSensitive() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxKeySensitive, true)
		c.Next()
	}
}
