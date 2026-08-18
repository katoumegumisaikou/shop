package address

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/response"
)

// Handler 收货地址模块 HTTP 处理器。
type Handler struct {
	svc *Service
}

// NewHandler 构造 Handler。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ---- 路由对照表 ----
//
// POST   /address/:id      → h.Create     // 注意：常规 REST 是 POST ""，这里 URL 带 :id
// GET    /address/:id      → h.Get
// GET    /address          → h.List
// DELETE /address/:id      → h.Delete
// PUT    /address          → h.Update     // 注意：常规 REST 是 PUT /:id，ID 从 body 传
// POST   /address/default  → h.SetDefault
// POST   /address/decrypt-wx → h.DecryptWxAddress (本文件不实现)

// Get 查询单条地址。
func (h *Handler) Get(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	userID := c.GetInt64("user_id")
	resp, err := h.svc.Get(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// List 列出当前用户全部地址。
func (h *Handler) List(c *gin.Context) {
	userID := c.GetInt64("user_id")
	resp, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"items": resp})
}

// Create 新建地址。
//
// 路由是 POST /:id，但创建不需要 URL 上的 ID（:id 在这里忽略）；
// 若 URL 上有 :id 仅记录日志便于排查路由误用。
func (h *Handler) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	userID := c.GetInt64("user_id")
	resp, err := h.svc.Create(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Update 更新指定地址。
//
// 路由是 PUT ""，所以 ID 必须放在 body（updateReq.ID）；
// 若业务方误用 URL :id，这里给参数错误提示。
func (h *Handler) Update(c *gin.Context) {
	var req updateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if req.ID <= 0 {
		response.Error(c, errs.ErrParam.WithMsg("id 必填且必须 > 0"))
		return
	}
	userID := c.GetInt64("user_id")
	resp, err := h.svc.Update(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Delete 删除指定地址。
func (h *Handler) Delete(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Delete(c.Request.Context(), userID, id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// SetDefault 把指定地址设为默认。
//
// 路由是 POST /default，body 里带 id。
func (h *Handler) SetDefault(c *gin.Context) {
	var req defaultReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if req.ID <= 0 {
		response.Error(c, errs.ErrParam.WithMsg("id 必填且必须 > 0"))
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.SetDefault(c.Request.Context(), userID, req.ID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// DecryptWxAddress 解密微信地址（占位实现，TODO: 调用微信加密数据解密接口）。
func (h *Handler) DecryptWxAddress(c *gin.Context) {
	response.Error(c, errs.ErrServiceDegraded.WithMsg("DecryptWxAddress 暂未实现"))
}

// ---- helpers ----

// parseID 从 URL :id 解析 int64，失败时返回 errs.ErrParam。
func parseID(c *gin.Context, name string) (int64, error) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.ErrParam.WithMsg("id 非法")
	}
	return id, nil
}