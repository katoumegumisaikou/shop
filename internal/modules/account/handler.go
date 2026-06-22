package account

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
	pkgjwt "shop/internal/pkg/jwt"
	"shop/internal/pkg/response"
)

// Handler 账号模块 HTTP 处理器。
type Handler struct {
	svc                 *Service
	jwtCfg              pkgjwt.JwtConfig
	isProd              bool
	h5RedirectAllowList []string // 完整 URL 白名单，H5Callback state 跳转允许名单
}

// NewHandler 构造 Handler。
// h5RedirectAllowList：完整 URL 列表，可为 nil/空，为空时 H5Callback 仅允许 state 是以 "/" 开头的同源相对路径。
func NewHandler(svc *Service, jwtCfg pkgjwt.JwtConfig, isProd bool, h5RedirectAllowList []string) *Handler {
	return &Handler{svc: svc, jwtCfg: jwtCfg, isProd: isProd, h5RedirectAllowList: h5RedirectAllowList}
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

	refreshMaxAge := int(h.jwtCfg.RefreshExpiration.Seconds())
	if refreshMaxAge <= 0 {
		refreshMaxAge = int(result.ExpiresIn)
	}

	// access_token 设为 HttpOnly + Secure cookie，禁止 JS 读取
	c.SetCookie(
		"access_token",
		result.AccessToken,
		int(result.ExpiresIn),
		"/",
		"",       // domain 由 nginx 注入，此处留空
		h.isProd, // secure（HTTPS only）
		true,     // httpOnly
	)
	// refresh_token 同样走 cookie
	c.SetCookie(
		"refresh_token",
		result.RefreshToken,
		refreshMaxAge,
		"/api/v1/c/auth/refresh",
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
