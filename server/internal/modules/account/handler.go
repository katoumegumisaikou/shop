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
	accessStr, _ := utils.GetJWTTokenFromCtx(c, adminAccessTokenCookieName)
	refreshStr, _ := utils.GetJWTTokenFromCtx(c, adminRefreshTokenCookieName)

	if err := h.svc.AdminLoginout(c.Request.Context(), accessStr, refreshStr); err != nil {
		response.Error(c, err)
		return
	}

	c.SetCookie(
		adminAccessTokenCookieName,
		"",
		-1,
		adminAccessTokenCookiePath,
		"",
		h.isProd,
		true,
	)
	c.SetCookie(
		adminRefreshTokenCookieName,
		"",
		-1,
		adminRefreshTokenCookiePath,
		"",
		h.isProd,
		true,
	)
	response.OK(c, nil)
}

func (h *Handler) AdminGetMe(c *gin.Context) {
	adminID := c.GetInt64("admin_id")
	if adminID == 0 {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}
	resp, err := h.svc.AdminGetMe(c.Request.Context(), adminID)
	if err != nil {
		response.Error(c, errs.ErrServiceDegraded)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) ListAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	admins, total, err := h.svc.ListAdmins(c.Request.Context(), page, size)
	if err != nil {
		response.Error(c, err)
		return
	}

	list := make([]AdminResp, len(admins))
	for i, a := range admins {
		list[i] = *toAdminResp(&a)
	}

	response.OK(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": size,
	})
}

func (h *Handler) UpdateAdmin(c *gin.Context) {
	adminID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	var req UpdateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	if err := h.svc.UpdateAdmin(c.Request.Context(), adminID, &req); err != nil {
		response.Error(c, err)
		return
	}

	// 返回更新后的管理员信息
	resp, err := h.svc.AdminGetMe(c.Request.Context(), adminID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) CreateAdmin(c *gin.Context) {
	var req CreateAdminReq
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.svc.CreateAdmin(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) DisableAdmin(c *gin.Context) {
	adminID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.DisableAdmin(c.Request.Context(), adminID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) EnableAdmin(c *gin.Context) {
	adminID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.EnableAdmin(c.Request.Context(), adminID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) ResetAdminPwd(c *gin.Context) {
	var req ResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	adminID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.ResetAdminPwd(c.Request.Context(), adminID, req.Password); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) ListRoles(c *gin.Context) {
	roles, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, roles)
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	role, err := h.svc.CreateRole(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, role)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	var req UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	role, err := h.svc.UpdateRole(c.Request.Context(), roleID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, role)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.DeleteRole(c.Request.Context(), roleID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.svc.ListPermissions(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, permissions)
}

func (h *Handler) AdminListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	users, total, err := h.svc.AdminListUsers(c.Request.Context(), page, size)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"list": users, "total": total, "page": page, "page_size": size})
}

func (h *Handler) AdminCreateUser(c *gin.Context) {
	var req AdminCreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	user, err := h.svc.AdminCreateUser(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *Handler) AdminGetUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	user, err := h.svc.AdminGetUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *Handler) AdminDisableUser(c *gin.Context) {
	h.adminUpdateUserStatus(c, false)
}

func (h *Handler) AdminEnableUser(c *gin.Context) {
	h.adminUpdateUserStatus(c, true)
}

func (h *Handler) adminUpdateUserStatus(c *gin.Context, enable bool) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	if enable {
		err = h.svc.AdminEnableUser(c.Request.Context(), userID)
	} else {
		err = h.svc.AdminDisableUser(c.Request.Context(), userID)
	}
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) AdminRechargeBalance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrInternal.WithMsg("错误用户id"))
		return
	}

	var req AdminRechargeBalanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	operatorID := c.GetInt64("admin_id")
	if operatorID == 0 {
		response.Error(c, errs.ErrUnauth)
		return
	}

	if err := h.svc.AdminRechargeBalance(c.Request.Context(), id, req.AmountCents, operatorID, req.Remark); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *Handler) AdminListBalanceLogs(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.ErrParam)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.svc.AdminListBalanceLogs(c.Request.Context(), userID, page, size)
	if err != nil {
		response.Error(c, err)
		return
	}

	list := make([]BalanceLogResp, len(logs))
	for i, l := range logs {
		list[i] = BalanceLogResp{
			ID:                 l.ID,
			ChangeCents:        l.ChangeCents,
			Type:               l.Type,
			RefType:            l.RefType,
			RefID:              l.RefID,
			BalanceBeforeCents: l.BalanceBeforeCents,
			BalanceAfterCents:  l.BalanceAfterCents,
			Remark:             l.Remark,
			CreatedAt:          l.CreatedAt,
		}
	}

	response.OK(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": size,
	})
}
