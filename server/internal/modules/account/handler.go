package account

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/response"
	"shop/internal/pkg/utils"
)

const (
	// C 端令牌 cookie
	userAccessTokenCookieName  = "access_token"
	userRefreshTokenCookieName = "refresh_token"
	userAccessTokenCookiePath  = "/api/v1/c"
	userRefreshTokenCookiePath = "/api/v1/c/auth"

	// Admin 端令牌 cookie
	adminAccessTokenCookieName  = "access_token"
	adminRefreshTokenCookieName = "refresh_token"
	adminAccessTokenCookiePath  = "/api/v1/admin"
	adminRefreshTokenCookiePath = "/api/v1/admin/auth"
)

// Handler 账号模块 HTTP 处理器。
type Handler struct {
	svc                 *Service
	userJwtCfg          pkgjwt.JwtConfig
	adminJwtCfg         pkgjwt.JwtConfig
	isProd              bool
	h5RedirectAllowList []string // 完整 URL 白名单，H5Callback state 跳转允许名单
}

// NewHandler 构造 Handler。
// h5RedirectAllowList：完整 URL 列表，可为 nil/空，为空时 H5Callback 仅允许 state 是以 "/" 开头的同源相对路径。
func NewHandler(
	svc *Service,
	userJwtCfg pkgjwt.JwtConfig,
	adminJwtCfg pkgjwt.JwtConfig,
	isProd bool,
	h5RedirectAllowList []string,
) *Handler {
	return &Handler{
		svc:                 svc,
		userJwtCfg:          userJwtCfg,
		adminJwtCfg:         adminJwtCfg,
		isProd:              isProd,
		h5RedirectAllowList: h5RedirectAllowList,
	}
}

func (h *Handler) MpLogin(c *gin.Context) {
	var req MpLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam.WithMsg("invalid mp login request"))
		return
	}

	result, err := h.svc.MpLogin(c.Request.Context(), req.Code)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, ToMpLoginResponse(result))
}

// H5GetOAuthURL 获取公众号 OAuth2 授权 URL，返回 redirect_url。
func (h *Handler) H5GetOAuthURL(c *gin.Context) {
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	if redirectURI == "" {
		response.Error(c, errs.ErrParam)
		return
	}
	authURL := h.svc.H5GetOAuthURL(c.Request.Context(), redirectURI, state)
	response.OK(c, gin.H{"url": authURL})
}

// H5Callback 公众号 OAuth2 回调，将 access_token 设为 HttpOnly cookie 后重定向
func (h *Handler) H5Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Error(c, errs.ErrSessionExpired)
		return
	}
	state := c.Query("state")

	result, err := h.svc.H5Callback(c.Request.Context(), code)
	if err != nil {
		response.Error(c, err)
		return
	}

	refreshMaxAge := int(h.userJwtCfg.RefreshExpiration.Seconds())
	if refreshMaxAge <= 0 {
		refreshMaxAge = int(result.ExpiresIn)
	}

	// access_token 设为 HttpOnly + Secure cookie，禁止 JS 读取
	c.SetCookie(
		userAccessTokenCookieName,
		result.AccessToken,
		int(result.ExpiresIn),
		userAccessTokenCookiePath,
		"",       // domain 由 nginx 注入，此处留空
		h.isProd, // secure（HTTPS only）
		true,     // httpOnly
	)
	// refresh_token 同样走 cookie
	c.SetCookie(
		userRefreshTokenCookieName,
		result.RefreshToken,
		refreshMaxAge,
		userRefreshTokenCookiePath,
		"",
		h.isProd,
		true,
	)

	c.Redirect(http.StatusFound, h.safeH5Redirect(state))

}

func (h *Handler) safeH5Redirect(state string) string {
	fallback := "/"
	if state == "" {
		return fallback
	}

	for _, allowed := range h.h5RedirectAllowList {
		if allowed == state {
			return state
		}
	}

	lower := strings.ToLower(state)
	// 仅接受以 / 开头的相对路径
	if !strings.HasPrefix(lower, "/") {
		return fallback
	}
	if strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "/\\") {
		return fallback
	}
	if strings.Contains(lower, "http:") || strings.Contains(lower, "https:") {
		return fallback
	}

	return state
}

func (h *Handler) BindPhone(c *gin.Context) {
	var req BindPhoneReq
	err := c.ShouldBind(&req)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	userID := c.GetInt64("user_id")
	if userID == 0 {
		response.Error(c, errs.ErrParam)
		return
	}

	// 业务逻辑
	if err = h.svc.BindPhone(c.Request.Context(), userID, req.EncryptedData, req.IV); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)

}

// refresh 刷新token
func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshTokenReq
	err := c.ShouldBind(&req)
	if err != nil {
		// 尝试从cookie中取出
		var ok bool
		req.RefreshToken, ok = utils.GetJWTTokenFromCtx(c, userRefreshTokenCookieName)
		if !ok {
			response.Error(c, errs.ErrUnauth)
			return
		}
	}

	result, err := h.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, errs.ErrUnauth)
		return
	}

	c.SetCookie(userAccessTokenCookieName, result.AccessToken, int(result.ExpiresIn), userAccessTokenCookiePath, "", h.isProd, true)
	c.SetCookie(userRefreshTokenCookieName, result.RefreshToken, int(result.RefreshExpiresIn), userRefreshTokenCookiePath, "", h.isProd, true)
	response.OK(c, gin.H{
		"access_token": result.AccessToken,
		"expires_in":   result.ExpiresIn,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	accessToken, ok := utils.GetJWTTokenFromCtx(c, userAccessTokenCookieName)
	if !ok {
		response.Error(c, errs.ErrUnauth)
		return
	}

	refreshToken, _ := getCookieToken(c, userRefreshTokenCookieName)
	if err := h.svc.Logout(c.Request.Context(), accessToken, refreshToken); err != nil {
		response.Error(c, err)
		return
	}
	// 清除 cookie
	c.SetCookie(userAccessTokenCookieName, "", -1, userAccessTokenCookiePath, "", h.isProd, true)
	c.SetCookie(userRefreshTokenCookieName, "", -1, userRefreshTokenCookiePath, "", h.isProd, true)

	response.OK(c, nil)
}

func getCookieToken(c *gin.Context, name string) (string, bool) {
	token, err := c.Cookie(name)
	if err != nil {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

// SendSmsCode 验证码的发送是通过http直接返回，目前没有接入短信平台
func (h *Handler) SendSmsCode(c *gin.Context) {
	var req SmsReq
	err := c.ShouldBind(&req)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	code, err := h.svc.SendSmsCode(c.Request.Context(), req.Phone, req.Purpose)
	if err != nil {
		response.Error(c, err)
		return
	}

	// 目前没有接入短信平台，先凑合着用吧
	response.OK(c, SmsResp{
		Code:      code,
		ExpiresIn: int64(smsCodeTTL.Seconds()),
	})
}

func (h *Handler) RegisterByPhone(c *gin.Context) {
	var req PhoneRegisterReq
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	result, err := h.svc.RegisterByPhone(c.Request.Context(), req.Phone, req.Password, req.Code)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.SetCookie(userAccessTokenCookieName, result.AccessToken, int(result.ExpiresIn), userAccessTokenCookiePath, "", h.isProd, true)
	c.SetCookie(userRefreshTokenCookieName, result.RefreshToken, int(result.RefreshExpiresIn), userRefreshTokenCookiePath, "", h.isProd, true)
	response.OK(c, nil)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordReq
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	accessToken, _ := utils.GetJWTTokenFromCtx(c, userAccessTokenCookieName)
	refreshToken, _ := getCookieToken(c, userRefreshTokenCookieName)

	result, err := h.svc.ResetPassword(c.Request.Context(), req.Phone, req.Password, req.Code, accessToken, refreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.SetCookie(userAccessTokenCookieName, result.AccessToken, int(result.ExpiresIn), userAccessTokenCookiePath, "", h.isProd, true)
	c.SetCookie(userRefreshTokenCookieName, result.RefreshToken, int(result.RefreshExpiresIn), userRefreshTokenCookiePath, "", h.isProd, true)
	response.OK(c, nil)
}

func (h *Handler) PhoneLogin(c *gin.Context) {
	var req PhoneLoginReq
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	result, err := h.svc.PhoneLogin(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}

	c.SetCookie(userAccessTokenCookieName, result.AccessToken, int(result.AccessExpiresIn), userAccessTokenCookiePath, "", h.isProd, true)
	c.SetCookie(userRefreshTokenCookieName, result.RefreshToken, int(result.RefreshExpiresIn), userRefreshTokenCookiePath, "", h.isProd, true)
	response.OK(c, result)
}

func (h *Handler) GetMe(c *gin.Context) {
	userID, isExist := c.Get("user_id")
	if !isExist {
		response.Error(c, errs.ErrUnauth)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Error(c, errs.ErrInternal)
		return
	}

	result, err := h.svc.GetMe(c.Request.Context(), uid)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, result)
}

// RequestDeactivate 申请注销。
func (h *Handler) RequestDeactivate(c *gin.Context) {
	var req DeactivateReq
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, err)
		return
	}
	userID := c.GetInt64("user_id")
	if userID == 0 {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}
	if err := h.svc.RequestDeactivate(c.Request.Context(), userID, req.Reason); err != nil {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) CancelDeactivate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}

	if err := h.svc.CancelDeactivate(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) GetMyBalance(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}

	balance_cents, err := h.svc.GetBalance(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, errs.ErrInternal)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total, err := h.svc.ListBalanceLogs(c.Request.Context(), userID, page, size)
	if err != nil {
		response.Error(c, errs.ErrInternal)
		return
	}
	response.OK(c, gin.H{
		"balance_cents": balance_cents,
		"logs":          logs,
		"total":         total,
		"page":          page,
		"page_size":     size,
	})
}

func (h *Handler) UpdateMe(c *gin.Context) {
	var req UpdateMeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam.WithMsg("请求参数格式错误"))
		return
	}

	userID := c.GetInt64("user_id")
	if userID == 0 {
		response.Error(c, errs.ErrUnauth)
		return
	}

	result, err := h.svc.UpdateMe(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// ------------------------------ Admin 端 ------------------

func (h *Handler) AdminGetCaptcha(c *gin.Context) {
	resp, err := h.svc.AdminGetCaptcha(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) AdminLogin(c *gin.Context) {
	var req AdminLoginReq
	err := c.ShouldBind(&req)
	if err != nil {
		response.Error(c, err)
		return
	}

	var result *LoginResult
	if result, err = h.svc.AdminLogin(c.Request.Context(), &req, c.ClientIP(), c.Request.UserAgent()); err != nil {
		response.Error(c, err)
		return
	}

	resp := &AdminLoginResponse{
		AdminID:          result.UserID,
		AccessToken:      result.AccessToken,
		AccessExpiresIn:  result.ExpiresIn,
		RefreshToken:     result.RefreshToken,
		RefreshExpiresIn: result.RefreshExpiresIn,
		TokenType:        "Bearer",
	}
	c.SetCookie(
		adminAccessTokenCookieName,
		resp.AccessToken,
		int(resp.AccessExpiresIn),
		adminAccessTokenCookiePath,
		"",
		h.isProd,
		true,
	)
	c.SetCookie(
		adminRefreshTokenCookieName,
		resp.RefreshToken,
		int(resp.RefreshExpiresIn),
		adminRefreshTokenCookiePath,
		"",
		h.isProd,
		true,
	)
	response.OK(c, resp)
}

func (h *Handler) AdminLoginout(c *gin.Context) {

}
