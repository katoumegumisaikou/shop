package product

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{svc: service}
}

func (h *Handler) ListCategories(c *gin.Context) {
	resp, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// ListProducts C 端商品列表。
func (h *Handler) ListProducts(c *gin.Context) {
	var req ProductListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, total, err := h.svc.ListProducts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{
		"products":  resp,
		"total":     total,
		"page":      req.Page,
		"page-size": req.PageSize,
	})
}

// ListProducts C 端热点商品列表。
func (h *Handler) ListHotProducts(c *gin.Context) {
	resp, err := h.svc.ListHotProducts(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

func (h *Handler) GetProduct(c *gin.Context) {
	// 路由是 /products/:id，用 Param 取路径参数（Query 取的是 ?id= 查询串）
	productID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	// UserOptionalAuth 登录时写入 user_id，未登录时为 0
	userID := c.GetInt64("user_id")
	resp, err := h.svc.GetProduct(c.Request.Context(), productID, userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AddFavorite 收藏商品。
func (h *Handler) AddFavorite(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.AddFavorite(c.Request.Context(), c.GetInt64("user_id"), productID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// RemoveFavorite 取消收藏。
func (h *Handler) RemoveFavorite(c *gin.Context) {
	productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || productID <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.RemoveFavorite(c.Request.Context(), c.GetInt64("user_id"), productID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// ListFavorites 收藏列表（分页）。
func (h *Handler) ListFavorites(c *gin.Context) {
	var req ListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, total, err := h.svc.ListFavorites(c.Request.Context(), c.GetInt64("user_id"), req.Page, req.PageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"products": resp, "total": total})
}

// GetViewHistory 最近浏览列表（分页）。
func (h *Handler) GetViewHistory(c *gin.Context) {
	var req ListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, total, err := h.svc.GetViewHistory(c.Request.Context(), c.GetInt64("user_id"), req.Page, req.PageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"products": resp, "total": total})
}
