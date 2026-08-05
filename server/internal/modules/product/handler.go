package product

import (
	"github.com/gin-gonic/gin"

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
