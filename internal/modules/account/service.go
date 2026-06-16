package account

import (
	"github.com/redis/go-redis/v9"

	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/wxlogin"
)

// Service 账号服务。
type Service struct {
	userRepo UserRepo
	rdb      *redis.Client
	userCfg  pkgjwt.JwtConfig
	adminCfg pkgjwt.JwtConfig
	wxMP     wxlogin.WxLoginClient // 小程序
	wxOA     wxlogin.WxLoginClient // 公众号
	env      string                // APP_ENV：dev/staging/prod
}

// NewService 构造 Service。
func NewService(
	userRepo UserRepo,
	rdb *redis.Client,
	cfg *pkgjwt.JwtConfig,
	wxMP, wxOA wxlogin.WxLoginClient,
) *Service {
	return &Service{
		userRepo: userRepo,
		rdb:      rdb,
		userCfg:  pkgjwt.GetUserConfig(),
		adminCfg: pkgjwt.GetAdminConfig(),
		wxMP:     wxMP,
		wxOA:     wxOA,
		// env:      cfg.App.Env,
	}
}
