package account

// MpLoginRequest 是小程序登录请求体。
type MpLoginRequest struct {
	Code string `json:"code" binding:"required"` // wx.login 返回的临时 code
}

// MpLoginResponse 是小程序登录响应体。
type MpLoginResponse struct {
	AccessToken  string `json:"access_token"`  // 访问接口使用的短期 token
	RefreshToken string `json:"refresh_token"` // 刷新 access token 使用的长期 token
	TokenType    string `json:"token_type"`    // token 类型，固定为 Bearer
	ExpiresIn    int64  `json:"expires_in"`    // access token 剩余有效期，单位秒
	UserID       int64  `json:"user_id"`       // 当前登录用户 ID
}

// ToMpLoginResponse 把领域层登录结果转换为 HTTP 响应 DTO。
func ToMpLoginResponse(result *MpLoginResult) MpLoginResponse {
	if result == nil {
		return MpLoginResponse{}
	}
	return MpLoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    result.ExpiresIn,
		UserID:       result.UserID,
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
