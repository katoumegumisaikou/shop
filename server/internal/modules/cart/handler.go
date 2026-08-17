package cart

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/response"
)

// Handler 购物车模块 HTTP 处理器。
type Handler struct{ svc *Service }

// NewHandler 构造 Handler。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List 获取购物车列表。
func (h *Handler) List(c *gin.Context) {
	userID := c.GetInt64("user_id")
	resp, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// Add 添加商品到购物车。
func (h *Handler) Add(c *gin.Context) {
	var req AddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Add(c.Request.Context(), userID, req.SkuID.Int64(), req.Qty); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Update 修改数量。
func (h *Handler) Update(c *gin.Context) {
	id, ok := mustParamID(c, "id")
	if !ok {
		return
	}
	var req UpdateQtyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Update(c.Request.Context(), id, userID, req.Qty); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Delete 删除单条。
func (h *Handler) Delete(c *gin.Context) {
	id, ok := mustParamID(c, "id")
	if !ok {
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Delete(c.Request.Context(), id, userID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// BatchDelete 批量删除。
func (h *Handler) BatchDelete(c *gin.Context) {
	var req BatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	ids := make([]int64, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = id.Int64()
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.BatchDelete(c.Request.Context(), userID, ids); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// CleanInvalid 清除不可用条目。
func (h *Handler) CleanInvalid(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if err := h.svc.CleanInvalid(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Count 购物车数量。
func (h *Handler) Count(c *gin.Context) {
	userID := c.GetInt64("user_id")
	cnt, err := h.svc.Count(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"count": cnt})
}

// Precheck 下单前检查。
func (h *Handler) Precheck(c *gin.Context) {
	var req PrecheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.ErrParam)
		return
	}
	ids := make([]int64, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = id.Int64()
	}
	userID := c.GetInt64("user_id")
	resp, err := h.svc.Precheck(c.Request.Context(), userID, ids)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// mustParamID 解析路径参数 id，返回 (id, ok)。失败时直接写错误响应。
func mustParamID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.ErrParam)
		return 0, false
	}
	return id, true
}