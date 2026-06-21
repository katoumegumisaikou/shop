package account

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/wxlogin"
)

const (
	// sessionKeyTTL 微信 session_key 缓存时长（2 小时）。
	sessionKeyTTL = 2 * time.Hour
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

func (s *Service) checkMpLoginDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("account service is not initialized")
	}
	if s.userRepo == nil {
		return errs.ErrInternal.WithMsg("account user repository is not initialized")
	}
	if s.rdb == nil {
		return errs.ErrInternal.WithMsg("account redis client is not initialized")
	}
	if s.wxMP == nil {
		return errs.ErrInternal.WithMsg("wechat mini program client is not initialized")
	}
	if s.userCfg.JwtSecret == "" {
		return errs.ErrInternal.WithMsg("user jwt secret is not configured")
	}
	if s.userCfg.Expiration <= 0 {
		return errs.ErrInternal.WithMsg("user jwt expiration is not configured")
	}
	if s.userCfg.RefreshExpiration <= 0 {
		return errs.ErrInternal.WithMsg("user refresh jwt expiration is not configured")
	}
	return nil
}

// MpLoginResult 小程序登录结果。
type MpLoginResult struct {
	AccessToken  string
	RefreshToken string
	UserID       int64
	ExpiresIn    int64
}

// MpLogin 小程序登录（code2session → upsert → 签发 JWT）。
func (s *Service) MpLogin(ctx context.Context, code string) (*MpLoginResult, error) {
	if err := s.checkMpLoginDependencies(); err != nil {
		return nil, err
	}

	resp, err := s.wxMP.Code2Session(ctx, code)
	if err != nil {
		return nil, errs.ErrSessionExpired
	}

	id := snowflake.NextID()
	mp := resp.OpenID
	var unionID *string
	if resp.UnionID != "" {
		unionID = new(string)
		*unionID = resp.UnionID
	}

	newUser := &User{
		ID:       id,
		OpenidMP: &mp,
		Unionid:  unionID,
		Status:   "active",
	}
	if err := s.userRepo.UpsertByOpenidMP(ctx, newUser); err != nil {
		return nil, err
	}

	// 查看真实的 ID
	u, err := s.userRepo.FindByOpenidMP(ctx, mp)
	if err != nil {
		return nil, err
	}

	// 写入 redis 缓存其 SessionKey 用于后续解码用户信息
	skKey := fmt.Sprintf("mp:%d", u.ID)
	_, err = s.rdb.Set(ctx, skKey, resp.SessionKey, sessionKeyTTL).Result()
	if err != nil {
		return nil, err
	}

	// 发放 token
	return s.signUserToken(u.ID)
}

// signUserToken 签署用户 Token
func (s *Service) signUserToken(id int64) (*MpLoginResult, error) {
	// accessToken 签署
	userCfg := s.userCfg
	accessClaim := &pkgjwt.Claims{
		Sub:  id,
		Type: "user",
		JTI:  uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(userCfg.Expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	accessToken, err := pkgjwt.Sign(userCfg.JwtSecret, *accessClaim)
	if err != nil {
		return nil, err
	}

	// refreshToken 签署
	refreshClaim := &pkgjwt.Claims{
		Sub:  id,
		Type: "user",
		JTI:  uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(userCfg.RefreshExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshToken, err := pkgjwt.Sign(userCfg.JwtSecret, *refreshClaim)
	if err != nil {
		return nil, err
	}
	return &MpLoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       id,
		ExpiresIn:    int64(s.userCfg.Expiration.Seconds()),
	}, nil
}
