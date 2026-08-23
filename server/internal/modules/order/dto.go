package order

import (
	"shop/internal/pkg/types"
)

// CreateOrderReq 下单请求。
type CreateOrderReq struct {
	AddressID         types.Int64Str   `json:"address_id"           binding:"required"`
	Items             []OrderItemReq   `json:"items"                binding:"required,min=1"`
	BuyerRemark       string           `json:"buyer_remark"`
	IdempotencyKey    string           `json:"idempotency_key"`
	FreightTemplateID *types.Int64Str  `json:"freight_template_id"`
	UseBalance        bool             `json:"use_balance"`
	FromCartItemIDs   []types.Int64Str `json:"from_cart_item_ids"` // 购物车下单时传，下单成功后删
	CouponID          *types.Int64Str  `json:"coupon_id"`          // 用户券 id（user_coupon.id）
	PointUsed         int64            `json:"point_used"`         // 使用积分数（1 积分 = 1 分）
}

// OrderItemReq 单行商品请求。
type OrderItemReq struct {
	SkuID types.Int64Str `json:"sku_id" binding:"required"`
	Qty   int            `json:"qty"    binding:"required,min=1,max=999"`
}

// CreateOrderResp 下单回复
type CreateOrderResp struct {
	OrderID int64  `json:"order_id"`
	OrderNo string `json:"order_no"`
}
