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
	"shop/internal/pkg/utils"
	"shop/internal/pkg/wxlogin"
)

const (
	// sessionKeyTTL 微信 session_key 缓存时长（2 小时）。
	sessionKeyTTL = 2 * time.Hour
	// smsRateTTL 短信发送频率限制（60 秒内最多 1 次）。
	smsRateTTL = 60 * time.Second
	// smsCodeTTL 短信验证码有效期（5 分钟）。
	smsCodeTTL = 5 * time.Minute
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

// checkMpLoginDependencies 检查小程序登录链路运行所需的依赖。
//
// 这些错误都属于服务启动或依赖注入配置问题，不能暴露给用户具体细节，
// 因此统一包装成 ErrInternal，交给 handler 层转换为 500 响应。
func (s *Service) checkMpLoginDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("账号服务未初始化")
	}
	if s.userRepo == nil {
		return errs.ErrInternal.WithMsg("账号用户仓储未初始化")
	}
	if s.rdb == nil {
		return errs.ErrInternal.WithMsg("账号 Redis 客户端未初始化")
	}
	if s.wxMP == nil {
		return errs.ErrInternal.WithMsg("微信小程序客户端未初始化")
	}
	if s.userCfg.JwtSecret == "" {
		return errs.ErrInternal.WithMsg("用户 JWT 密钥未配置")
	}
	if s.userCfg.Expiration <= 0 {
		return errs.ErrInternal.WithMsg("用户 JWT 有效期未配置")
	}
	if s.userCfg.RefreshExpiration <= 0 {
		return errs.ErrInternal.WithMsg("用户刷新 JWT 有效期未配置")
	}
	return nil
}

func (s *Service) checkH5CallbackDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("账号服务未初始化")
	}
	if s.userRepo == nil {
		return errs.ErrInternal.WithMsg("账号用户仓储未初始化")
	}
	if s.wxOA == nil {
		return errs.ErrInternal.WithMsg("微信公众号客户端未初始化")
	}
	if s.userCfg.JwtSecret == "" {
		return errs.ErrInternal.WithMsg("用户 JWT 密钥未配置")
	}
	if s.userCfg.Expiration <= 0 {
		return errs.ErrInternal.WithMsg("用户 JWT 有效期未配置")
	}
	if s.userCfg.RefreshExpiration <= 0 {
		return errs.ErrInternal.WithMsg("用户刷新 JWT 有效期未配置")
	}
	return nil
}

func (s *Service) checkBindPhoneDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("账号服务未初始化")
	}
	if s.userRepo == nil {
		return errs.ErrInternal.WithMsg("账号用户仓储未初始化")
	}
	if s.rdb == nil {
		return errs.ErrInternal.WithMsg("账号 Redis 客户端未初始化")
	}
	if s.wxMP == nil {
		return errs.ErrInternal.WithMsg("微信小程序客户端未初始化")
	}
	return nil
}

func (s *Service) checkUserTokenDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("账号服务未初始化")
	}
	if s.rdb == nil {
		return errs.ErrInternal.WithMsg("账号 Redis 客户端未初始化")
	}
	if s.userCfg.JwtSecret == "" {
		return errs.ErrInternal.WithMsg("用户 JWT 密钥未配置")
	}
	return nil
}

func (s *Service) checkRefreshTokenDependencies() error {
	if err := s.checkUserTokenDependencies(); err != nil {
		return err
	}
	if s.userCfg.Expiration <= 0 {
		return errs.ErrInternal.WithMsg("用户 JWT 有效期未配置")
	}
	if s.userCfg.RefreshExpiration <= 0 {
		return errs.ErrInternal.WithMsg("用户刷新 JWT 有效期未配置")
	}
	return nil
}

func (s *Service) checkSmsCodeDependencies() error {
	if s == nil {
		return errs.ErrInternal.WithMsg("账号服务未初始化")
	}
	if s.rdb == nil {
		return errs.ErrInternal.WithMsg("账号 Redis 客户端未初始化")
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

// H5GetOAuthURL 获取公众号 OAuth2 授权 URL。
func (s *Service) H5GetOAuthURL(_ context.Context, redirectURI, state string) string {
	return s.wxOA.GetOAuthURL(redirectURI, state)
}

type H5CallbackResult = MpLoginResult

// H5Callback 公众号 OAuth2 回调，upsert user，返回 token
func (s *Service) H5Callback(ctx context.Context, code string) (*H5CallbackResult, error) {
	if err := s.checkH5CallbackDependencies(); err != nil {
		return nil, err
	}

	resp, err := s.wxOA.OAuthCode2Token(ctx, code)
	if err != nil {
		return nil, errs.ErrSessionExpired
	}

	id := snowflake.NextID()
	openid := resp.OpenID
	var unionid *string
	if resp.UnionID != "" {
		unionid = new(string)
		*unionid = resp.UnionID
	}

	u := &User{
		ID:       id,
		OpenidH5: &openid,
		Unionid:  unionid,
		Source:   "h5",
		Status:   "active",
	}

	if err := s.userRepo.UpsertByOpenidH5(ctx, u); err != nil {
		return nil, errs.ErrInternal
	}

	found, err := s.userRepo.FindByOpenidH5(ctx, openid)
	if err != nil || found == nil {
		return nil, errs.ErrInternal
	}

	return s.signUserToken(found.ID)
}

// BindPhone 解密微信数据包并绑定手机号。
func (s *Service) BindPhone(ctx context.Context, userID int64, encryptedData, iv string) error {
	if err := s.checkBindPhoneDependencies(); err != nil {
		return err
	}

	skKey := fmt.Sprintf("mp:%d", userID)
	sessionKey, err := s.rdb.Get(ctx, skKey).Result()
	if err != nil {
		return errs.ErrInternal
	}

	data, err := s.wxMP.DecryptUserData(sessionKey, encryptedData, iv)
	if err != nil {
		return errs.ErrInternal.WithMsg("解码失败")
	}

	phone, ok := data["purePhoneNumber"].(string)
	if !ok || phone == "" {
		return errs.ErrPhoneFormat
	}

	// 数据库查找是否被其他活跃账号绑定
	count, err := s.userRepo.CountActiveByPhoneExclude(ctx, phone, userID)
	if err != nil {
		return errs.ErrInternal
	} else if count > 0 {
		// 手机号已经使用过
		return errs.ErrPhoneBound
	}

	if err := s.userRepo.Update(ctx, userID, map[string]any{"phone": phone}); err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*MpLoginResult, error) {
	if err := s.checkRefreshTokenDependencies(); err != nil {
		return nil, err
	}

	// refresh token 一次性：立即加入黑名单
	claims, err := s.parseUserToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if err := s.blacklistTokenClaims(ctx, claims); err != nil {
		return nil, err
	}

	return s.signUserToken(claims.Sub)
}

func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if err := s.checkUserTokenDependencies(); err != nil {
		return err
	}

	accessClaims, err := s.parseUserToken(accessToken)
	if err != nil {
		return err
	}
	if err := s.blacklistTokenClaims(ctx, accessClaims); err != nil {
		return err
	}

	if refreshToken == "" {
		return nil
	}
	refreshClaims, err := s.parseUserToken(refreshToken)
	if err != nil {
		return err
	}
	return s.blacklistTokenClaims(ctx, refreshClaims)
}

func (s *Service) parseUserToken(token string) (*pkgjwt.Claims, error) {
	claims, err := pkgjwt.Parse(s.userCfg.JwtSecret, token)
	if err != nil || claims.Type != "user" {
		return nil, errs.ErrUnauth
	}
	return claims, nil
}

func (s *Service) blacklistTokenClaims(ctx context.Context, claims *pkgjwt.Claims) error {
	if claims == nil || claims.JTI == "" || claims.ExpiresAt == nil {
		return errs.ErrUnauth
	}
	exp := time.Until(claims.ExpiresAt.Time)
	if exp <= 0 {
		return errs.ErrUnauth
	}

	tokenKey := fmt.Sprintf("jwt:bl:%s", claims.JTI)
	ok, err := s.rdb.SetNX(ctx, tokenKey, "1", exp).Result()
	if err != nil {
		return errs.ErrInternal
	}
	if !ok {
		return errs.ErrUnauth
	}
	return nil
}

func (s *Service) SendSmsCode(ctx context.Context, phone, purpose string) (string, error) {
	if err := s.checkSmsCodeDependencies(); err != nil {
		return "", err
	}

	// 速率限制
	rateKey := fmt.Sprintf("shop:sms:rate:%s:%s", purpose, phone)
	ok, err := s.rdb.SetNX(ctx, rateKey, "1", smsRateTTL).Result()
	if err != nil {
		return "", errs.ErrInternal
	}
	if !ok {
		return "", errs.ErrRateLimit.WithMsg("发送过于频繁，请 60 秒后重试")
	}

	// 生成随机数
	code, err := utils.GenerateSmsCode()
	if err != nil {
		_ = s.rdb.Del(ctx, rateKey).Err()
		return "", errs.ErrInternal
	}

	codeKey := fmt.Sprintf("shop:sms:code:%s:%s", purpose, phone)
	_, err = s.rdb.Set(ctx, codeKey, code, smsCodeTTL).Result()
	if err != nil {
		_ = s.rdb.Del(ctx, rateKey).Err()
		return "", errs.ErrInternal
	}
	return code, nil
}
