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
func ToMpLoginResponse(result *MpLoginResult) MpLoginResponse {
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
