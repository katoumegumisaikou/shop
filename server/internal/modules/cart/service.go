package cart

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"go.uber.org/zap"

	"shop/internal/modules/product"
	"shop/internal/pkg/errs"
	"shop/internal/pkg/types"
)

const maxCartRows = 100

// Service 购物车服务。
// 与 xu-shop 不同：没有独立的 skuRepo 和 stockClient —— SKU/Product 状态通过
// CartRepo.FindSKUWithProduct（一次 SQL JOIN）取得；可用库存直接走 DB 兜底
// (Stock - LockedStock)，不引入 stock 包。
type Service struct {
	repo        CartRepo
	productRepo product.ProductRepo
	logger      *zap.Logger
}

// NewService 构造 Service。
func NewService(repo CartRepo, productRepo product.ProductRepo, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		repo:        repo,
		productRepo: productRepo,
		logger:      logger,
	}
}

// List 购物车列表（含可用性）。
func (s *Service) List(ctx context.Context, userID int64) (*CartListResp, error) {
	rows, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]CartItemResp, len(rows))
	for i, r := range rows {
		p := CartProductResp{
			ID:            types.Int64Str(r.ProductID),
			CategoryID:    types.Int64Str(r.ProductCategoryID),
			Title:         r.ProductTitle,
			Subtitle:      r.ProductSubtitle,
			MainImage:     r.ProductMainImage,
			Status:        r.ProductStatus,
			Sales:         r.ProductSales,
			Tags:          json.RawMessage(r.ProductTags),
			PriceMinCents: r.ProductPriceMinCents,
			PriceMaxCents: r.ProductPriceMaxCents,
		}
		skuImage := r.SkuImage
		if skuImage == "" {
			skuImage = r.ProductMainImage
		}

		item := CartItemResp{
			ID:                 types.Int64Str(r.ID),
			SkuID:              types.Int64Str(r.SkuID),
			ProductID:          types.Int64Str(r.ProductID),
			Product:            p,
			ProductTitle:       r.ProductTitle,
			SkuImage:           skuImage,
			SkuAttrs:           r.SkuAttrs,
			Qty:                r.Qty,
			SnapshotPriceCents: r.SnapshotPriceCents,
			CurrentPriceCents:  r.SkuPriceCents,
			AvailableStock:     s.resolveAvailableStock(r.SkuStock, r.SkuLockedStock),
			IsAvailable:        true,
		}
		// 可用性检查
		reason := checkAvailability(r, item.AvailableStock)
		if reason != "" {
			item.IsAvailable = false
			item.UnavailableReason = reason
		}
		items[i] = item
	}
	return &CartListResp{Items: items, Total: int64(len(items))}, nil
}

// checkAvailability 检查购物车条目可用性，返回不可用原因（空表示可用）。
func checkAvailability(r CartItemDetail, availableStock int) string {
	if r.ProductID == 0 || r.SkuStatus == "" {
		return "sku_not_found"
	}
	if r.ProductDeleted {
		return "product_deleted"
	}
	if r.ProductStatus != "onsale" {
		return "product_offsale"
	}
	if r.SkuStatus != "active" {
		return "sku_disabled"
	}
	if availableStock < r.Qty {
		return "sku_oos"
	}
	return ""
}

// resolveAvailableStock 计算可用库存：DB 兜底（Stock - LockedStock）。
func (s *Service) resolveAvailableStock(stock, locked int) int {
	available := stock - locked
	if available < 0 {
		return 0
	}
	return available
}

// Add 添加商品到购物车。
func (s *Service) Add(ctx context.Context, userID, skuID int64, qty int) error {
	sku, err := s.repo.FindSKUWithProduct(ctx, skuID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.ErrNotFound.WithMsg("SKU 不存在")
		}
		return errs.ErrInternal
	}
	if sku.SkuStatus != "active" {
		return errs.ErrParam.WithMsg("SKU 已下架")
	}
	if sku.ProductDeleted {
		return errs.ErrNotFound.WithMsg("商品不存在")
	}
	if sku.ProductStatus != "onsale" {
		return errs.ErrParam.WithMsg("商品未上架")
	}

	// 检查购物车总行数 ≤ 100
	cnt, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return errs.ErrInternal
	}
	if cnt >= maxCartRows {
		return errs.ErrConflict.WithMsg("购物车已满（最多 100 种商品）")
	}

	return s.repo.Upsert(ctx, userID, skuID, qty, sku.SkuPriceCents)
}

// Update 修改购物车条目数量。
func (s *Service) Update(ctx context.Context, id, userID int64, qty int) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.ErrNotFound
		}
		return errs.ErrInternal
	}
	if item.UserID != userID {
		return errs.ErrForbidden
	}
	sku, err := s.repo.FindSKUWithProduct(ctx, item.SkuID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.ErrParam.WithMsg("商品规格已失效")
		}
		return errs.ErrInternal
	}
	if sku.SkuStatus != "active" {
		return errs.ErrParam.WithMsg("商品规格已失效")
	}
	if sku.ProductDeleted {
		return errs.ErrParam.WithMsg("商品已失效")
	}
	if sku.ProductStatus != "onsale" {
		return errs.ErrParam.WithMsg("商品已下架")
	}

	available := s.resolveAvailableStock(sku.SkuStock, sku.SkuLockedStock)
	if available < qty {
		if available <= 0 {
			return errs.ErrParam.WithMsg("当前无库存")
		}
		return errs.ErrParam.WithMsg(fmt.Sprintf("库存不足，当前最多可购买 %d 件", available))
	}

	return s.repo.Update(ctx, id, qty, sku.SkuPriceCents)
}

// Delete 删除单条购物车条目。
func (s *Service) Delete(ctx context.Context, id, userID int64) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.ErrNotFound
		}
		return errs.ErrInternal
	}
	if item.UserID != userID {
		return errs.ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}

// BatchDelete 批量删除（校验归属）。
func (s *Service) BatchDelete(ctx context.Context, userID int64, ids []int64) error {
	return s.repo.DeleteByUserAndIDs(ctx, userID, ids)
}

// CleanInvalid 清除不可用条目。
func (s *Service) CleanInvalid(ctx context.Context, userID int64) error {
	rows, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return errs.ErrInternal
	}
	var invalidIDs []int64
	for _, r := range rows {
		available := s.resolveAvailableStock(r.SkuStock, r.SkuLockedStock)
		if checkAvailability(r, available) != "" {
			invalidIDs = append(invalidIDs, r.ID)
		}
	}
	if len(invalidIDs) == 0 {
		return nil
	}
	return s.repo.BatchDelete(ctx, invalidIDs)
}

// Count 购物车条目数。
func (s *Service) Count(ctx context.Context, userID int64) (int64, error) {
	cnt, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return 0, errs.ErrInternal
	}
	return cnt, nil
}

// Precheck 下单前检查选中条目（价格/库存冲突）。
func (s *Service) Precheck(ctx context.Context, userID int64, ids []int64) (*PrecheckResp, error) {
	items, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errs.ErrInternal
	}

	resp := &PrecheckResp{
		Conflicts: make([]PrecheckConflict, 0),
		OK:        true,
	}
	for _, item := range items {
		if item.UserID != userID {
			continue
		}
		sku, skuErr := s.repo.FindSKUWithProduct(ctx, item.SkuID)
		if skuErr != nil {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "sku_not_found",
			})
			resp.OK = false
			continue
		}
		if sku.ProductDeleted {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "product_deleted",
			})
			resp.OK = false
			continue
		}
		if sku.ProductStatus != "onsale" {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "product_offsale",
			})
			resp.OK = false
			continue
		}
		if sku.SkuStatus != "active" {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "sku_disabled",
			})
			resp.OK = false
			continue
		}

		// 价格变动
		if sku.SkuPriceCents != item.SnapshotPriceCents {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "price_changed",
			})
			resp.OK = false
		}

		// 库存不足
		available := s.resolveAvailableStock(sku.SkuStock, sku.SkuLockedStock)
		if available < item.Qty {
			resp.Conflicts = append(resp.Conflicts, PrecheckConflict{
				CartItemID: types.Int64Str(item.ID),
				SkuID:      types.Int64Str(item.SkuID),
				Reason:     "stock_insufficient",
			})
			resp.OK = false
		}
	}
	return resp, nil
}