package account

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-redis/redis_rate/v10"

	"shop/internal/middleware"
	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/types"
	"shop/internal/pkg/utils"
	"shop/internal/pkg/wxlogin"
)

const (
	// sessionKeyTTL 微信 session_key 缓存时长（2 小时）。
	sessionKeyTTL = 2 * time.Hour
	// smsCodeTTL 短信验证码有效期（5 分钟）。
	smsCodeTTL = 5 * time.Minute
	// loginFailTTL 登录失败时间。
	loginFailTTL = 5 * time.Minute
	// loginFailLockTTL 登录失败锁定时间
	loginFailLockTTL = 5 * time.Minute
	// loginFailThreshold 登录失败最大次数，超过后触发锁定。
	loginFailThreshold = 4
)

// Service 账号服务。
type Service struct {
	userRepo UserRepo
	rdb      *redis.Client
	limiter  *redis_rate.Limiter
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
		limiter:  redis_rate.NewLimiter(rdb),
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

// MpLoginResult 登录结果。
type MpLoginResult struct {
	AccessToken      string
	RefreshToken     string
	UserID           int64
	ExpiresIn        int64 // access_token 剩余有效期，单位秒
	RefreshExpiresIn int64 // refresh_token 剩余有效期，单位秒
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
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		UserID:           id,
		ExpiresIn:        int64(s.userCfg.Expiration.Seconds()),
		RefreshExpiresIn: int64(s.userCfg.RefreshExpiration.Seconds()),
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

// blacklistTokenClaims 将 JWT 加入 Redis 黑名单，使其在剩余有效期内不可用。
//
// 黑名单 key 为 jwt:bl:{jti}，TTL 为 token 剩余有效期，到期自动释放。
// 登出或 refresh token 一次性使用时会调用此函数。
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

	// 速率限制：每分钟同一手机号+目的仅允许 1 次
	rateKey := fmt.Sprintf("shop:sms:rate:%s:%s", purpose, phone)
	if err := middleware.RateLimiter(ctx, s.limiter, rateKey, redis_rate.PerMinute(1)); err != nil {
		return "", err
	}

	// 生成随机数
	code, err := utils.GenerateSmsCode()
	if err != nil {
		_ = s.rdb.Del(ctx, rateKey).Err()
		return "", errs.ErrInternal
	}

	codeKey := fmt.Sprintf("shop:code:%s:%s", purpose, phone)
	_, err = s.rdb.Set(ctx, codeKey, code, smsCodeTTL).Result()
	if err != nil {
		_ = s.rdb.Del(ctx, rateKey).Err()
		return "", errs.ErrInternal
	}
	return code, nil
}

// RegisterByPhone 注册新用户。
//
// 验证验证码和密码强度后创建用户并签发 token。
func (s *Service) RegisterByPhone(ctx context.Context, phone, password, code string) (*MpLoginResult, error) {
	if err := s.checkUserTokenDependencies(); err != nil {
		return nil, err
	}
	// 检验验证码
	if isExist, err := s.verifySmsCode(ctx, phone, code, "register"); err != nil || !isExist {
		return nil, err
	}
	_, _ = s.rdb.Del(ctx, fmt.Sprintf("shop:code:%s:%s", "register", phone)).Result()

	// 验证密码强度
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	// 确认手机号未被使用
	cnt, err := s.userRepo.CountByPhone(ctx, phone)
	if err != nil {
		return nil, err
	} else if cnt > 0 {
		return nil, errs.ErrConflict.WithMsg("手机号已经被使用")
	}

	// 哈希加密密钥
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, errs.ErrInternal
	}
	hashStr := string(hash)

	nickName := "用户" + phone
	u := &User{
		ID:           snowflake.NextID(),
		Phone:        &phone,
		NickName:     &nickName,
		PasswordHash: &hashStr,
		Status:       "active",
		Source:       "phone",
	}
	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.signUserToken(u.ID)
}

// ResetPassword 重置密码。
//
// 验证验证码和密码强度后更新密码并签发 token。
func (s *Service) ResetPassword(ctx context.Context, phone, password, code, accessToken, refreshToken string) (*MpLoginResult, error) {
	if err := s.checkUserTokenDependencies(); err != nil {
		return nil, err
	}

	// 检验验证码
	if isExist, err := s.verifySmsCode(ctx, phone, code, "reset"); err != nil || !isExist {
		return nil, err
	}
	_, _ = s.rdb.Del(ctx, fmt.Sprintf("shop:code:%s:%s", "reset", phone)).Result()

	// 验证密码强度
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	// 确认账号存在并获取用户信息
	user, err := s.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, errs.ErrNotFound.WithMsg("账号不存在，无法重置密码")
	}

	// 哈希加密密钥
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, errs.ErrInternal
	}
	hashStr := string(hash)

	// 修改密码
	if err := s.userRepo.Update(ctx, user.ID, map[string]any{
		"password_hash": hashStr,
	}); err != nil {
		return nil, errs.ErrInternal.WithMsg("修改密码失败")
	}

	// 将之前token加入黑名单
	accessClaims, err := pkgjwt.Parse(s.userCfg.JwtSecret, accessToken)
	if err != nil {
		return nil, err
	}
	refreshClaims, err := pkgjwt.Parse(s.userCfg.JwtSecret, refreshToken)
	if err != nil {
		return nil, err
	}

	_ = s.blacklistTokenClaims(ctx, accessClaims)
	_ = s.blacklistTokenClaims(ctx, refreshClaims)

	return s.signUserToken(user.ID)
}

// 允许的密码特殊符号（键盘上可见的符号）
const passwordSymbols = "!#$%&'()*+-=`{|}~"

// validatePassword 校验密码强度。
//
//   - 至少 6 位
//   - 包含大写字母、小写字母、数字、符号中至少 2 类
func validatePassword(password string) error {
	if len(password) < 6 {
		return errs.ErrParam.WithMsg("密码至少 6 位")
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, ch := range password {
		switch {
		case 'A' <= ch && ch <= 'Z':
			hasUpper = true
		case 'a' <= ch && ch <= 'z':
			hasLower = true
		case '0' <= ch && ch <= '9':
			hasDigit = true
		case strings.ContainsRune(passwordSymbols, ch):
			hasSymbol = true
		default:
			// 不允许的字符（中文、emoji、控制字符等），视为非法
			return errs.ErrParam.WithMsg("密码包含不允许的特殊字符")
		}
	}

	categories := 0
	if hasUpper {
		categories++
	}
	if hasLower {
		categories++
	}
	if hasDigit {
		categories++
	}
	if hasSymbol {
		categories++
	}
	if categories < 2 {
		return errs.ErrParam.WithMsg("密码需包含大写字母、小写字母、数字、特殊字符中至少 2 类")
	}
	return nil
}

func (s *Service) verifySmsCode(ctx context.Context, phone, code, purpose string) (bool, error) {
	key := fmt.Sprintf("shop:code:%s:%s", purpose, phone)
	redisCode, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, errs.ErrRateLimit.WithMsg("验证码已过期")
	} else if err != nil {
		return false, errs.ErrInternal
	}
	if redisCode == code {
		return true, nil
	}
	return false, nil
}

func (s *Service) PhoneLogin(ctx context.Context, req *PhoneLoginReq) (*PhoneLoginResp, error) {
	// 查看用户是否被锁定
	lockKey := fmt.Sprintf("shop:lock:%s", req.Phone)
	exists, err := s.rdb.Exists(ctx, lockKey).Result()
	if err != nil {
		return nil, err
	} else if exists == 1 {
		return nil, errs.ErrAccountLocked
	}

	// 查找用户
	u, err := s.userRepo.FindByPhone(ctx, req.Phone)
	if err != nil {
		return nil, errs.ErrParam.WithMsg("密码或账号错误")
	} else if u.Status != "active" {
		return nil, errs.ErrAccountLocked
	}

	// 检验用户密码
	if u.PasswordHash == nil {
		return nil, errs.ErrParam.WithMsg("用户未设置密码，请尝试其他方式登录")
	}
	err = bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(req.Password))
	if err != nil {
		s.incLoginFail(ctx, req.Phone)
		return nil, errs.ErrParam.WithMsg("密码或账号错误")
	}

	// 清空登录失败次数
	failKey := fmt.Sprintf("shop:fail:%s", req.Phone)
	_ = s.rdb.Del(ctx, failKey)

	r, err := s.signUserToken(u.ID)
	if err != nil {
		return nil, err
	}

	result := &PhoneLoginResp{
		AccessToken:      r.AccessToken,
		RefreshToken:     r.RefreshToken,
		AccessExpiresIn:  r.ExpiresIn,
		RefreshExpiresIn: r.RefreshExpiresIn,
		User: &UserResp{
			ID:       types.Int64Str(u.ID),
			Phone:    u.Phone,
			Nickname: u.NickName,
			Avatar:   u.Avatar,
			Status:   u.Status,
		},
	}
	return result, nil

}

// incLoginFail 增加登录失败计数，超限则锁定，实现指数退避
func (s *Service) incLoginFail(ctx context.Context, phone string) error {
	failKey := fmt.Sprintf("shop:fail:%s", phone)
	pipe := s.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, failKey)
	_ = pipe.Expire(ctx, failKey, loginFailTTL)
	_, _ = pipe.Exec(ctx)
	cnt := incrCmd.Val()

	// 按阶段锁定：达到阈值时锁定，每多一个阈值阶段指数递增
	if cnt >= loginFailThreshold {
		stage := cnt / loginFailThreshold
		lockTTL := time.Duration(math.Pow(2, float64(stage-1)) * float64(loginFailLockTTL))
		lockKey := fmt.Sprintf("shop:lock:%s", phone)
		if err := s.rdb.Set(ctx, lockKey, 1, lockTTL).Err(); err != nil {
			return err
		}
		// 延长失败计数 TTL，与锁定时间一致，避免解锁后仍残留旧计数
		_, _ = s.rdb.Expire(ctx, failKey, lockTTL).Result()
	}
	return nil
}
