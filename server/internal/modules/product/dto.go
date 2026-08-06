package product

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/types"
)

type CategoriesResp struct {
	ID        types.Int64Str   `json:"id"`
	ParentID  types.Int64Str   `json:"parent_id"`
	Name      string           `json:"name"`
	Icon      string           `json:"icon,omitempty"`
	Sort      int              `json:"sort"`
	Status    string           `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	Children  []CategoriesResp `json:"children,omitempty"`
}

// ToCategoriesResp 将 Category 实体转为 HTTP 响应 DTO。
// 不填充 Children——树形结构由调用方递归构建。
func ToCategoriesResp(c *Category) *CategoriesResp {
	if c == nil {
		return &CategoriesResp{}
	}
	return &CategoriesResp{
		ID:        types.Int64Str(c.ID),
		ParentID:  types.Int64Str(c.ParentID),
		Name:      c.Name,
		Icon:      c.Icon,
		Sort:      c.Sort,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
	}
}

// ProductListReq 商品列表查询参数。
type ProductListReq struct {
	CategoryID types.Int64Str `form:"category_id"`
	Status     string         `form:"status"`
	Keyword    string         `form:"keyword"`
	Sort       string         `form:"sort"      binding:"omitempty,oneof=latest popular price_asc price_desc hot"`
	Page       int            `form:"page"      binding:"min=0"`
	PageSize   int            `form:"page_size" binding:"min=0,max=50"`
	InStock    bool           `form:"in_stock"`
	IDs        string         `form:"ids"`
}

// ListReq 收藏 / 浏览历史列表的分页请求。
type ListReq struct {
	Page     int `form:"page"     binding:"min=0"`
	PageSize int `form:"page_size" binding:"min=0,max=50"`
}

func (r *ProductListReq) IDsList() ([]int64, error) {
	parts := strings.Split(r.IDs, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, errs.ErrParam.WithMsg("ids 包含非法 ID")
		}
		ids = append(ids, id)
	}
	if len(ids) > 20 {
		return nil, errs.ErrParam.WithMsg("ids 最多 20 个")
	}
	return ids, nil
}

// ProductResp 商品列表响应 DTO。
type ProductResp struct {
	ID                  types.Int64Str  `json:"id"`
	CategoryID          types.Int64Str  `json:"category_id"`
	CategoryName        *string         `json:"category_name,omitempty"`
	Title               string          `json:"title"`
	Subtitle            string          `json:"subtitle,omitempty"`
	MainImage           string          `json:"main_image"`
	Status              string          `json:"status"`
	Unit                string          `json:"unit"`
	IsVirtual           bool            `json:"is_virtual"`
	FreightTemplateID   *types.Int64Str `json:"freight_template_id,omitempty"`
	FreightTemplateName *string         `json:"freight_template_name,omitempty"`
	Sales               int             `json:"sales"`
	TotalStock          int             `json:"total_stock"`
	Sort                int             `json:"sort"`
	Tags                json.RawMessage `json:"tags"`
	PriceMinCents       int64           `json:"price_min_cents"`
	PriceMaxCents       int64           `json:"price_max_cents"`
	OnSaleAt            *time.Time      `json:"on_sale_at,omitempty"`
}

// ToProductResp 将 Product 实体转为 HTTP 响应 DTO。
//
// 说明：
//   - Sales 合并了真实销量 + 虚拟销量，避免向客户端暴露 virtual_sales
//   - CategoryName / FreightTemplateName / TotalStock 需要联表或聚合才能得到，
//     由调用方在获取到关联数据后自行填充
func ToProductResp(p *Product) *ProductResp {
	if p == nil {
		return &ProductResp{}
	}
	return &ProductResp{
		ID:                types.Int64Str(p.ID),
		CategoryID:        types.Int64Str(p.CategoryID),
		Title:             p.Title,
		Subtitle:          p.Subtitle,
		MainImage:         p.MainImage,
		Status:            p.Status,
		Unit:              p.Unit,
		IsVirtual:         p.IsVirtual,
		FreightTemplateID: toInt64StrPtr(p.FreightTemplateID),
		Sales:             p.Sales + p.VirtualSales, // 对外只暴露合并销量
		Sort:              p.Sort,
		Tags:              json.RawMessage(p.Tags),
		PriceMinCents:     p.PriceMinCents,
		PriceMaxCents:     p.PriceMaxCents,
		OnSaleAt:          p.OnSaleAt,
	}
}

// toInt64StrPtr 将 *int64 转为 *types.Int64Str。
func toInt64StrPtr(v *int64) *types.Int64Str {
	if v == nil {
		return nil
	}
	s := types.Int64Str(*v)
	return &s
}

// ProductDetailResp 商品详情响应 DTO。
type ProductDetailResp struct {
	ProductResp
	Images      json.RawMessage `json:"images"`
	VideoURL    string          `json:"video_url,omitempty"`
	DetailHTML  string          `json:"detail_html,omitempty"`
	DetailNodes json.RawMessage `json:"detail_nodes,omitempty"`
	Specs       []SpecResp      `json:"specs"`
	SKUs        []UserSKUResp   `json:"skus"`
	IsFavorite  bool            `json:"is_favorite"`
}

// SpecResp 规格响应。
type SpecResp struct {
	ID     types.Int64Str  `json:"id"`
	Name   string          `json:"name"`
	Sort   int             `json:"sort"`
	Values []SpecValueResp `json:"values"`
}

// SpecValueResp 规格值响应。
type SpecValueResp struct {
	ID    types.Int64Str `json:"id"`
	Value string         `json:"value"`
	Sort  int            `json:"sort"`
}

// c 端 请求 SKUResp SKU 响应。
type UserSKUResp struct {
	ID                 types.Int64Str  `json:"id"`
	ProductID          types.Int64Str  `json:"product_id"`
	Attrs              json.RawMessage `json:"attrs"`
	PriceCents         int64           `json:"price_cents"`
	OriginalPriceCents *int64          `json:"original_price_cents,omitempty"`
	Stock              int             `json:"stock"`
	LockedStock        int             `json:"locked_stock"`
	WeightG            int             `json:"weight_g"`
	Image              string          `json:"image,omitempty"`
	Status             string          `json:"status"`
}

// toUserSKUResp entity → UserSKUResp。
func toUserSKUResp(s *SKU) UserSKUResp {
	return UserSKUResp{
		ID:                 types.Int64Str(s.ID),
		ProductID:          types.Int64Str(s.ProductID),
		Attrs:              json.RawMessage(s.Attrs),
		PriceCents:         s.PriceCents,
		OriginalPriceCents: s.OriginalPriceCents,
		Stock:              s.Stock,
		LockedStock:        s.LockedStock,
		WeightG:            s.WeightG,
		Image:              s.Image,
		Status:             s.Status,
	}
}

// ---- 后台管理 DTO ----

// AdminCategoryReq 分类创建/更新请求。
type AdminCategoryReq struct {
	ParentID types.Int64Str `json:"parent_id"`
	Name     string         `json:"name"   binding:"required"`
	Icon     string         `json:"icon"`
	Sort     int            `json:"sort"`
	Status   string         `json:"status" binding:"omitempty,oneof=enabled disabled"`
}

// AdminSpecInput 后台商品规格输入（规格名 + 规格值列表）。
type AdminSpecInput struct {
	Name   string   `json:"name"   binding:"required"`
	Sort   int      `json:"sort"`
	Values []string `json:"values"`
}

// AdminSKUInput 后台 SKU 输入。
type AdminSKUInput struct {
	Attrs              map[string]string `json:"attrs"`
	PriceCents         int64             `json:"price_cents"`
	OriginalPriceCents *int64            `json:"original_price_cents"`
	Stock              int               `json:"stock"`
	WeightG            int               `json:"weight_g"`
	SkuCode            *string           `json:"sku_code"`
	Barcode            *string           `json:"barcode"`
	Image              string            `json:"image"`
	Status             string            `json:"status"`
	LowStockThreshold  int               `json:"low_stock_threshold"`
}

// AdminProductReq 后台商品创建/更新请求。
type AdminProductReq struct {
	CategoryID        types.Int64Str   `json:"category_id"  binding:"required"`
	Title             string           `json:"title"        binding:"required"`
	Subtitle          string           `json:"subtitle"`
	MainImage         string           `json:"main_image"   binding:"required"`
	Images            json.RawMessage  `json:"images"`
	VideoURL          string           `json:"video_url"`
	DetailHTML        string           `json:"detail_html"`
	DetailNodes       json.RawMessage  `json:"detail_nodes"`
	Unit              string           `json:"unit"`
	IsVirtual         bool             `json:"is_virtual"`
	FreightTemplateID *int64           `json:"freight_template_id"`
	Sort              int              `json:"sort"`
	Tags              json.RawMessage  `json:"tags"`
	Specs             []AdminSpecInput `json:"specs"`
	SKUs              []AdminSKUInput  `json:"skus"`
}

// AdminBatchStatusReq 批量上下架请求。
type AdminBatchStatusReq struct {
	IDs    []int64 `json:"ids"    binding:"required,min=1,max=100"`
	Status string  `json:"status" binding:"required,oneof=draft onsale offsale"`
}

// AdminSKUPriceInput 批量改价中的单条。
type AdminSKUPriceInput struct {
	SKUID      int64 `json:"sku_id"      binding:"required"`
	PriceCents int64 `json:"price_cents" binding:"required"`
}

// AdminBatchPriceReq 批量改价请求。
type AdminBatchPriceReq struct {
	Items []AdminSKUPriceInput `json:"items" binding:"required,min=1,max=100"`
}

// AdminProductResp 后台商品列表项（含虚拟销量等内部字段）。
type AdminProductResp struct {
	ProductResp
	VirtualSales int       `json:"virtual_sales"`
	CreatedAt    time.Time `json:"created_at"`
}

// toAdminProductResp Product 实体 → 后台列表 DTO。
func toAdminProductResp(p *Product) *AdminProductResp {
	base := ToProductResp(p)
	return &AdminProductResp{
		ProductResp:  *base,
		VirtualSales: p.VirtualSales,
		CreatedAt:    p.CreatedAt,
	}
}

// AdminSKUResp 后台 SKU 响应（含内部管理字段）。
type AdminSKUResp struct {
	ID                 types.Int64Str  `json:"id"`
	ProductID          types.Int64Str  `json:"product_id"`
	Attrs              json.RawMessage `json:"attrs"`
	PriceCents         int64           `json:"price_cents"`
	OriginalPriceCents *int64          `json:"original_price_cents,omitempty"`
	Stock              int             `json:"stock"`
	LockedStock        int             `json:"locked_stock"`
	WeightG            int             `json:"weight_g"`
	SkuCode            *string         `json:"sku_code,omitempty"`
	Barcode            *string         `json:"barcode,omitempty"`
	Image              string          `json:"image,omitempty"`
	Status             string          `json:"status"`
	LowStockThreshold  int             `json:"low_stock_threshold"`
}

// toAdminSKUResp SKU 实体 → 后台 SKU DTO。
func toAdminSKUResp(s *SKU) AdminSKUResp {
	return AdminSKUResp{
		ID:                 types.Int64Str(s.ID),
		ProductID:          types.Int64Str(s.ProductID),
		Attrs:              json.RawMessage(s.Attrs),
		PriceCents:         s.PriceCents,
		OriginalPriceCents: s.OriginalPriceCents,
		Stock:              s.Stock,
		LockedStock:        s.LockedStock,
		WeightG:            s.WeightG,
		SkuCode:            s.SkuCode,
		Barcode:            s.Barcode,
		Image:              s.Image,
		Status:             s.Status,
		LowStockThreshold:  s.LowStockThreshold,
	}
}

// AdminProductDetailResp 后台商品详情响应。
type AdminProductDetailResp struct {
	ProductResp
	VirtualSales int             `json:"virtual_sales"`
	Images       json.RawMessage `json:"images"`
	VideoURL     string          `json:"video_url,omitempty"`
	DetailHTML   string          `json:"detail_html,omitempty"`
	DetailNodes  json.RawMessage `json:"detail_nodes,omitempty"`
	Specs        []SpecResp      `json:"specs"`
	SKUs         []AdminSKUResp  `json:"skus"`
}
