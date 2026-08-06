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

// ---- 后台管理 ----

// AdminListCategories 后台分类列表。
func (h *Handler) AdminListCategories(c *gin.Context) {
	resp, err := h.svc.AdminListCategories(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminCreateCategory 创建分类。
func (h *Handler) AdminCreateCategory(c *gin.Context) {
	var req AdminCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.svc.AdminCreateCategory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminUpdateCategory 更新分类。
func (h *Handler) AdminUpdateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	var req AdminCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.svc.AdminUpdateCategory(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminDeleteCategory 删除分类。
func (h *Handler) AdminDeleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.AdminDeleteCategory(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// AdminListProducts 后台商品列表。
func (h *Handler) AdminListProducts(c *gin.Context) {
	var req ProductListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, total, err := h.svc.AdminListProducts(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"products": resp, "total": total})
}

// AdminCreateProduct 创建商品。
func (h *Handler) AdminCreateProduct(c *gin.Context) {
	var req AdminProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.svc.AdminCreateProduct(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminGetProduct 后台商品详情。
func (h *Handler) AdminGetProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	resp, err := h.svc.AdminGetProduct(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminUpdateProduct 更新商品。
func (h *Handler) AdminUpdateProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	var req AdminProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.svc.AdminUpdateProduct(c.Request.Context(), id, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminDeleteProduct 删除商品。
func (h *Handler) AdminDeleteProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.AdminDeleteProduct(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// AdminCopyProduct 复制商品。
func (h *Handler) AdminCopyProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	resp, err := h.svc.AdminCopyProduct(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}

// AdminOnSale 上架。
func (h *Handler) AdminOnSale(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.AdminOnSale(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// AdminOffSale 下架。
func (h *Handler) AdminOffSale(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, errs.ErrParam)
		return
	}
	if err := h.svc.AdminOffSale(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// AdminBatchStatus 批量上下架。
func (h *Handler) AdminBatchStatus(c *gin.Context) {
	var req AdminBatchStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.AdminBatchStatus(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// AdminBatchPrice 批量改价。
func (h *Handler) AdminBatchPrice(c *gin.Context) {
	var req AdminBatchPriceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.AdminBatchPrice(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
