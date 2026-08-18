package order

import (
	"github.com/gin-gonic/gin"

	"shop/internal/pkg/response"
)

// Handler 订单模块 HTTP 处理器。
type Handler struct {
	svc *Service
}

// NewHandler 构造 Handler。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create 创建订单。
func (h *Handler) Create(c *gin.Context) {
	userID := c.GetInt64("user_id")
	key := c.GetHeader("Idempotency-Key")
	var req CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	req.IdempotencyKey = key

	resp, err := h.svc.CreateOrder(c.Request.Context(), userID, &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, resp)
}
