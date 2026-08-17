package cart

import (
	"encoding/json"

	"shop/internal/pkg/types"
)

// CartListResp 购物车列表响应。
type CartListResp struct {
	Items []CartItemResp `json:"items"`
	Total int64          `json:"total"`
}

// CartProductResp 商品视图（嵌入 CartItemResp）。
type CartProductResp struct {
	ID            types.Int64Str  `json:"id"`
	CategoryID    types.Int64Str  `json:"category_id"`
	Title         string          `json:"title"`
	Subtitle      string          `json:"subtitle,omitempty"`
	MainImage     string          `json:"main_image"`
	Status        string          `json:"status"`
	Sales         int             `json:"sales"`
	Tags          json.RawMessage `json:"tags"`
	PriceMinCents int64           `json:"price_min_cents"`
	PriceMaxCents int64           `json:"price_max_cents"`
}

// CartItemResp 单条购物车响应。
type CartItemResp struct {
	ID                 types.Int64Str  `json:"id"`
	SkuID              types.Int64Str  `json:"sku_id"`
	ProductID          types.Int64Str  `json:"product_id"`
	Product            CartProductResp `json:"product"`
	ProductTitle       string          `json:"product_title"`
	SkuImage           string          `json:"sku_image"`
	SkuAttrs           json.RawMessage `json:"sku_attrs"`
	Qty                int             `json:"qty"`
	SnapshotPriceCents int64           `json:"snapshot_price_cents"`
	CurrentPriceCents  int64           `json:"current_price_cents"`
	AvailableStock     int             `json:"available_stock"`
	IsAvailable        bool            `json:"is_available"`
	UnavailableReason  string          `json:"unavailable_reason,omitempty"`
}

// PrecheckResp 下单前检查响应。
type PrecheckResp struct {
	Conflicts []PrecheckConflict `json:"conflicts"`
	OK        bool               `json:"ok"`
}

// PrecheckConflict 冲突明细。
type PrecheckConflict struct {
	CartItemID types.Int64Str `json:"cart_item_id"`
	SkuID      types.Int64Str `json:"sku_id"`
	Reason     string         `json:"reason"`
}

// AddReq 添加商品到购物车请求体。
type AddReq struct {
	SkuID types.Int64Str `json:"sku_id" binding:"required"`
	Qty   int            `json:"qty"    binding:"required,min=1,max=999"`
}

// UpdateQtyReq 修改数量请求体。
type UpdateQtyReq struct {
	Qty int `json:"qty" binding:"required,min=1,max=999"`
}

// BatchDeleteReq 批量删除请求体。
type BatchDeleteReq struct {
	IDs []types.Int64Str `json:"ids" binding:"required,min=1"`
}

// PrecheckReq 下单前检查请求体。
type PrecheckReq struct {
	IDs []types.Int64Str `json:"ids" binding:"required,min=1"`
}