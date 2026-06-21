package account

import (
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

func (h *Handler) MpLogin(ctx *gin.Context) {
	var req MpLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.Error(ctx, errs.ErrParam.WithMsg("invalid mp login request"))
		return
	}

	result, err := h.svc.MpLogin(ctx.Request.Context(), req.Code)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.OK(ctx, ToMpLoginResponse(result))
}
