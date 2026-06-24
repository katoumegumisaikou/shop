package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GetJWTTokenFromCtx 从 Gin Context 中提取 JWT。
//
// 优先读取 Authorization: Bearer <token>，没有 Authorization 时再读取指定 cookie。
// 如果 Authorization 存在但格式不合法，不会继续回退到 cookie，避免错误 header 被静默忽略。
func GetJWTTokenFromCtx(c *gin.Context, cookieName string) (string, bool) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader != "" {
		fields := strings.Fields(authHeader)
		if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") && fields[1] != "" {
			return fields[1], true
		}
		return "", false
	}

	if cookieName == "" {
		return "", false
	}

	token, err := c.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}
