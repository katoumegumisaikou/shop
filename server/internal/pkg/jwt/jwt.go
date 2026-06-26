package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 用户JWT配置
// C 端 access token 有效期默认 30 天 ，refresh token 有效期 90 天
var userJwtConfig JwtConfig

// 管理员JWT配置
// 后台 access token 有效期默认 8 小时 ，refresh token 有效期 admin 7 天
var adminJwtConfig JwtConfig

// Claims 是业务 JWT 的载荷结构。
type Claims struct {
	Sub   int64    `json:"sub"`             // token 主体，一般是用户 ID 或管理员 ID
	Type  string   `json:"type"`            // token 类型或身份类型，例如 user/admin/access/refresh
	JTI   string   `json:"jti"`             // token 唯一 ID，可用于撤销 token 或防重放
	Roles []string `json:"roles,omitempty"` // 角色列表，为空时不写入 token
	Perms []string `json:"perms,omitempty"` // 权限点列表，为空时不写入 token

	// RegisteredClaims 是 JWT 标准声明，包含 exp、iat、nbf、iss、sub、aud、jti 等字段。
	jwt.RegisteredClaims
}

func Sign(secret string, claim Claims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	return t.SignedString([]byte(secret))
}

func Parse(secret string, token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("jwt: invalid token")
	}
	return c, nil
}

// JwtConfig 是 JWT 签发和解析配置。
type JwtConfig struct {
	Issuer            string
	JwtSecret         string        // 签名密钥
	JwtIssuer         string        // 签发方，对应标准声明 iss
	Expiration        time.Duration // access token 有效期
	RefreshExpiration time.Duration // refresh token 有效期
}

func GetUserConfig() JwtConfig {
	return userJwtConfig
}

func GetAdminConfig() JwtConfig {
	return adminJwtConfig
}

// InitJwtConfig 初始化用户端和管理员端两套 JWT 配置。
//
// 配置文件的读取和解析放在应用启动层完成，这里只接收已经解析好的配置。
func InitJwtConfig(userCfg, adminCfg JwtConfig) {
	userJwtConfig = withDefaultUserConfig(userCfg)
	adminJwtConfig = withDefaultAdminConfig(adminCfg)
}

func withDefaultUserConfig(cfg JwtConfig) JwtConfig {
	cfg = withCommonDefaultConfig(cfg)
	if cfg.Expiration == 0 {
		cfg.Expiration = 30 * 24 * time.Hour
	}
	if cfg.RefreshExpiration == 0 {
		cfg.RefreshExpiration = 90 * 24 * time.Hour
	}
	return cfg
}

func withDefaultAdminConfig(cfg JwtConfig) JwtConfig {
	cfg = withCommonDefaultConfig(cfg)
	if cfg.Expiration == 0 {
		cfg.Expiration = 8 * time.Hour
	}
	if cfg.RefreshExpiration == 0 {
		cfg.RefreshExpiration = 7 * 24 * time.Hour
	}
	return cfg
}

func withCommonDefaultConfig(cfg JwtConfig) JwtConfig {
	if cfg.JwtSecret == "" {
		cfg.JwtSecret = "dev-secret"
	}
	if cfg.JwtIssuer == "" {
		cfg.JwtIssuer = "shop"
	}
	return cfg
}
