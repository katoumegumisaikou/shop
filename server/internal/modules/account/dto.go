package account

import "time"
import "shop/internal/pkg/types"

// MpLoginRequest 是小程序登录请求体。
type MpLoginRequest struct {
	Code string `json:"code" binding:"required"` // wx.login 返回的临时 code
}

// MpLoginResponse 是小程序登录响应体。
type MpLoginResponse struct {
	AccessToken      string `json:"access_token"`       // 访问接口使用的短期 token
	RefreshToken     string `json:"refresh_token"`      // 刷新 access token 使用的长期 token
	TokenType        string `json:"token_type"`         // token 类型，固定为 Bearer
	AccessExpiresIn  int64  `json:"expires_in"`         // access token 剩余有效期，单位秒
	RefreshExpiresIn int64  `json:"refresh_expires_in"` // refresh token 剩余有效期，单位秒
	UserID           int64  `json:"user_id"`            // 当前登录用户 ID
}

// ToMpLoginResponse 把领域层登录结果转换为 HTTP 响应 DTO。
func ToMpLoginResponse(result *LoginResult) MpLoginResponse {
	if result == nil {
		return MpLoginResponse{}
	}
	return MpLoginResponse{
		AccessToken:     result.AccessToken,
		RefreshToken:    result.RefreshToken,
		TokenType:       "Bearer",
		AccessExpiresIn: result.ExpiresIn,
		UserID:          result.UserID,
	}
}

// BindPhoneReq 绑定手机号请求。
type BindPhoneReq struct {
	EncryptedData string `json:"encrypted_data" binding:"required"`
	IV            string `json:"iv"             binding:"required"`
}

// RefreshTokenReq 刷新 token 请求。
type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// SmsReq 发送短信验证码请求。
type SmsReq struct {
	Phone   string `json:"phone"   binding:"required,len=11,numeric"`
	Purpose string `json:"purpose" binding:"required,oneof=register reset"` // 区分注册和重置密码
}

// SmsResp 短信验证码响应。
type SmsResp struct {
	Code      string `json:"code"`       // 开发期通过 HTTP 返回的验证码
	ExpiresIn int64  `json:"expires_in"` // 验证码有效期，单位秒
}

// PhoneRegisterReq 手机号注册请求。
type PhoneRegisterReq struct {
	Phone    string `json:"phone"    binding:"required,mobile"`
	Code     string `json:"code"     binding:"required,len=6"`
	Password string `json:"password" binding:"required"`
}

// ResetPasswordReq 重置密码请求。
type ResetPasswordReq struct {
	Phone    string `json:"phone"    binding:"required,mobile"`
	Code     string `json:"code"     binding:"required,len=6"`
	Password string `json:"password" binding:"required"`
}

// PhoneLoginReq 手机号密码登录请求。
type PhoneLoginReq struct {
	Phone    string `json:"phone"    binding:"required,mobile"`
	Password string `json:"password" binding:"required"`
}

// PhoneLoginResp C 端手机号登录/注册响应。
type PhoneLoginResp struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccessExpiresIn  int64     `json:"expires_in"`         // access token 剩余有效期，单位秒
	RefreshExpiresIn int64     `json:"refresh_expires_in"` // refresh token 剩余有效期，单位秒
	User             *UserResp `json:"user"`
}

// UserResp 用户信息响应。
type UserResp struct {
	ID           types.Int64Str `json:"id"`
	Phone        *string        `json:"phone,omitempty"`
	Nickname     *string        `json:"nickname,omitempty"`
	Avatar       *string        `json:"avatar,omitempty"`
	Gender       int            `json:"gender"`
	Birthday     *time.Time     `json:"birthday,omitempty"`
	Status       string         `json:"status"`
	BalanceCents int64          `json:"balance_cents"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ToUserResp 将 User 实体转换为 HTTP 响应 DTO。
func ToUserResp(u *User) *UserResp {
	if u == nil {
		return nil
	}
	return &UserResp{
		ID:           types.Int64Str(u.ID),
		Phone:        u.Phone,
		Nickname:     u.NickName,
		Avatar:       u.Avatar,
		Gender:       u.Gender,
		Birthday:     u.Birthday,
		Status:       u.Status,
		BalanceCents: u.BalanceCents,
		CreatedAt:    u.CreatedAt,
	}
}

type DeactivateReq struct {
	Reason string `json:"reason"`
}

// UpdateMeReq 更新个人信息请求。
type UpdateMeReq struct {
	Nickname *string    `json:"nickname"` // 昵称
	Avatar   *string    `json:"avatar"`   // 头像 URL
	Gender   *int       `json:"gender"`   // 性别
	Birthday *time.Time `json:"birthday"` // 生日
}

// BalanceResp 余额响应。
type BalanceResp struct {
	BalanceCents int64 `json:"balance_cents"` // 余额，单位：分
}

// BalanceLogResp 余额流水响应。
type BalanceLogResp struct {
	ID                 int64     `json:"id"`
	ChangeCents        int64     `json:"change_cents"`         // 变动金额，单位：分
	Type               string    `json:"type"`                 // 变动类型
	RefType            *string   `json:"ref_type,omitempty"`   // 关联业务类型
	RefID              *int64    `json:"ref_id,omitempty"`     // 关联业务 ID
	BalanceBeforeCents int64     `json:"balance_before_cents"` // 变动前余额
	BalanceAfterCents  int64     `json:"balance_after_cents"`  // 变动后余额
	Remark             *string   `json:"remark,omitempty"`     // 备注
	CreatedAt          time.Time `json:"created_at"`
}

// BalanceLogPageResp 余额流水分页响应。
type BalanceLogPageResp struct {
	List  []BalanceLogResp `json:"list"`
	Total int64            `json:"total"`
}

// AdminCaptchaResp 验证码响应。
type AdminCaptchaResp struct {
	CaptchaID string `json:"captcha_id"`
	ImageB64  string `json:"captcha_b64"`
}

// AdminLoginReq 登录请求
type AdminLoginReq struct {
	Username    string `json:"username"     binding:"required"`
	Password    string `json:"password"     binding:"required"`
	CaptchaID   string `json:"captcha_id"   binding:"required"`
	CaptchaCode string `json:"captcha_code" binding:"required"`
}

// AdminLoginResponse 登录回复
type AdminLoginResponse struct {
	AccessToken      string `json:"access_token"`       // 访问接口使用的短期 token
	RefreshToken     string `json:"refresh_token"`      // 刷新 access token 使用的长期 token
	TokenType        string `json:"token_type"`         // token 类型，固定为 Bearer
	AccessExpiresIn  int64  `json:"expires_in"`         // access token 剩余有效期，单位秒
	RefreshExpiresIn int64  `json:"refresh_expires_in"` // refresh token 剩余有效期，单位秒
	AdminID          int64  `json:"admin_id"`           // 当前登录管理员 ID
}

// AdminResp 管理员信息响应。
type AdminResp struct {
	ID          types.Int64Str `json:"id"`
	Username    string         `json:"username"`
	RealName    *string        `json:"real_name,omitempty"`
	Phone       *string        `json:"phone,omitempty"`
	Status      string         `json:"status"`
	LastLoginAt *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Roles       []string       `json:"roles"`
	Perms       []string       `json:"perms"`
}

// toAdminResp 将 entity 转为响应 DTO。
func toAdminResp(a *Admin) *AdminResp {
	return &AdminResp{
		ID:          types.Int64Str(a.ID),
		Username:    a.Username,
		RealName:    a.RealName,
		Phone:       a.Phone,
		Status:      a.Status,
		LastLoginAt: a.LastLoginAt,
		CreatedAt:   a.CreatedAt,
	}
}

// UpdateAdminReq 更新管理员请求。
type UpdateAdminReq struct {
	RealName *string `json:"real_name"`
	Phone    *string `json:"phone"`
	Status   *string `json:"status"`
}

// CreateAdminReq 创建管理员请求。
type CreateAdminReq struct {
	Username string           `json:"username"  binding:"required"`
	Password string           `json:"password"  binding:"required,min=6"`
	RealName *string          `json:"real_name"`
	Phone    *string          `json:"phone"`
	RoleIDs  []types.Int64Str `json:"role_ids"`
}
