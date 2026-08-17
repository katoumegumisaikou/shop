package cart

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"shop/internal/modules/product"
	"shop/internal/pkg/errs"
)

// ---- mock CartRepo ----

type mockCartRepo struct {
	items       []CartItem
	details     []CartItemDetail
	upsertFn    func(userID, skuID int64, qty int, price int64) error
	countByUser int64
	skus        map[int64]*CartSKUInfo
}

func (m *mockCartRepo) FindByUserID(_ context.Context, _ int64) ([]CartItemDetail, error) {
	return m.details, nil
}
func (m *mockCartRepo) FindByID(_ context.Context, id int64) (*CartItem, error) {
	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockCartRepo) Upsert(_ context.Context, userID, skuID int64, qty int, price int64) error {
	if m.upsertFn != nil {
		return m.upsertFn(userID, skuID, qty, price)
	}
	return nil
}
func (m *mockCartRepo) Update(_ context.Context, _ int64, _ int, _ int64) error { return nil }
func (m *mockCartRepo) Delete(_ context.Context, _ int64) error                { return nil }
func (m *mockCartRepo) BatchDelete(_ context.Context, _ []int64) error        { return nil }
func (m *mockCartRepo) DeleteByUserAndIDs(_ context.Context, _ int64, _ []int64) error {
	return nil
}
func (m *mockCartRepo) CountByUser(_ context.Context, _ int64) (int64, error) {
	return m.countByUser, nil
}
func (m *mockCartRepo) FindByIDs(_ context.Context, ids []int64) ([]CartItem, error) {
	var res []CartItem
	for _, item := range m.items {
		for _, id := range ids {
			if item.ID == id {
				res = append(res, item)
			}
		}
	}
	return res, nil
}
func (m *mockCartRepo) FindSKUWithProduct(_ context.Context, id int64) (*CartSKUInfo, error) {
	if info, ok := m.skus[id]; ok {
		return info, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// ---- mock ProductRepo ----
// 实现 shop 的 product.ProductRepo 接口（仅 cart 用到的子集返回真实数据，
// 其他方法保持 stub 返回 nil/零值，确保编译通过）。

type mockProductRepo struct {
	products map[int64]product.Product
}

func (m *mockProductRepo) ListByIDs(_ context.Context, _ []int64) ([]*product.Product, error) {
	return nil, nil
}
func (m *mockProductRepo) Search(_ context.Context, _ product.ProductFilter) ([]*product.Product, int, error) {
	return nil, 0, nil
}
func (m *mockProductRepo) GetByID(_ context.Context, id int64) (*product.Product, error) {
	if p, ok := m.products[id]; ok {
		return &p, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockProductRepo) ListSpecs(_ context.Context, _ int64) ([]*product.ProductSpec, error) {
	return nil, nil
}
func (m *mockProductRepo) ListSpecValues(_ context.Context, _ []int64) ([]*product.ProductSpecValue, error) {
	return nil, nil
}
func (m *mockProductRepo) ListSKUs(_ context.Context, _ int64) ([]*product.SKU, error) {
	return nil, nil
}
func (m *mockProductRepo) CreateWithDetails(_ context.Context, _ *product.Product, _ []*product.ProductSpec, _ []*product.SKU) error {
	return nil
}
func (m *mockProductRepo) UpdateBase(_ context.Context, _ *product.Product) error { return nil }
func (m *mockProductRepo) DeleteByID(_ context.Context, _ int64) error            { return nil }
func (m *mockProductRepo) UpdateStatus(_ context.Context, _ []int64, _ string, _ *time.Time) error {
	return nil
}
func (m *mockProductRepo) ReplaceDetails(_ context.Context, _ int64, _ []*product.ProductSpec, _ []*product.SKU) error {
	return nil
}
func (m *mockProductRepo) UpdateSKUPrices(_ context.Context, _ map[int64]int64) error {
	return nil
}

// ---- 辅助构造 ----

func newSKUInfo(skuID, productID int64, price int64, skuStatus, prodStatus string, stock, locked int) *CartSKUInfo {
	return &CartSKUInfo{
		SkuID:          skuID,
		SkuPriceCents:  price,
		SkuStatus:      skuStatus,
		SkuStock:       stock,
		SkuLockedStock: locked,
		ProductID:      productID,
		ProductStatus:  prodStatus,
		ProductDeleted: false,
	}
}

func newTestService(repo CartRepo, count int64) *Service {
	cartRepo, _ := repo.(*mockCartRepo)
	if cartRepo == nil {
		cartRepo = &mockCartRepo{countByUser: count}
	}
	if cartRepo.skus == nil {
		cartRepo.skus = map[int64]*CartSKUInfo{
			10: newSKUInfo(10, 1, 1000, "active", "onsale", 100, 0),
		}
	}
	if cartRepo.countByUser == 0 && count > 0 {
		cartRepo.countByUser = count
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		1: {ID: 1, Status: "onsale"},
	}}
	return NewService(cartRepo, prodRepo, nil)
}

// ---- 测试用例 ----

// TestAdd_MergeExisting 重复加同一 SKU 应触发 UPSERT（合并数量）。
func TestAdd_MergeExisting(t *testing.T) {
	upsertCalled := 0
	repo := &mockCartRepo{
		countByUser: 1,
		skus: map[int64]*CartSKUInfo{
			10: newSKUInfo(10, 1, 1000, "active", "onsale", 100, 0),
		},
		upsertFn: func(_, _ int64, _ int, _ int64) error {
			upsertCalled++
			return nil
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		1: {ID: 1, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	if err := svc.Add(context.Background(), 100, 10, 2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if upsertCalled != 1 {
		t.Fatalf("expected upsert called 1 time, got %d", upsertCalled)
	}
}

// TestAdd_ExceedLimit 超过 100 行应被拒绝。
func TestAdd_ExceedLimit(t *testing.T) {
	repo := &mockCartRepo{
		countByUser: maxCartRows, // 已满
		skus: map[int64]*CartSKUInfo{
			10: newSKUInfo(10, 1, 1000, "active", "onsale", 100, 0),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		1: {ID: 1, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	err := svc.Add(context.Background(), 100, 10, 1)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	ae, ok := err.(*errs.AppError)
	if !ok || ae.Code != errs.ErrConflict.Code {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// TestPrecheck_PriceChanged 快照价格与当前价不一致时应返回冲突。
func TestPrecheck_PriceChanged(t *testing.T) {
	repo := &mockCartRepo{
		items: []CartItem{
			{ID: 1, UserID: 100, SkuID: 10, Qty: 2, SnapshotPriceCents: 900}, // 快照 900，当前 1000
		},
		skus: map[int64]*CartSKUInfo{
			10: newSKUInfo(10, 1, 1000, "active", "onsale", 100, 0),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		1: {ID: 1, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	resp, err := svc.Precheck(context.Background(), 100, []int64{1})
	if err != nil {
		t.Fatalf("Precheck error: %v", err)
	}
	if resp.OK {
		t.Fatal("expected OK=false")
	}
	if len(resp.Conflicts) == 0 {
		t.Fatal("expected conflicts")
	}
	if resp.Conflicts[0].Reason != "price_changed" {
		t.Fatalf("expected price_changed, got %s", resp.Conflicts[0].Reason)
	}
}

// TestPrecheck_StockInsufficient 库存不足时应返回 stock_insufficient 冲突。
func TestPrecheck_StockInsufficient(t *testing.T) {
	repo := &mockCartRepo{
		items: []CartItem{
			{ID: 2, UserID: 100, SkuID: 20, Qty: 10, SnapshotPriceCents: 500},
		},
		skus: map[int64]*CartSKUInfo{
			// Stock=5, LockedStock=3 → available=2 < qty=10
			20: newSKUInfo(20, 2, 500, "active", "onsale", 5, 3),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		2: {ID: 2, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	resp, err := svc.Precheck(context.Background(), 100, []int64{2})
	if err != nil {
		t.Fatalf("Precheck error: %v", err)
	}
	if resp.OK {
		t.Fatal("expected OK=false")
	}
	found := false
	for _, c := range resp.Conflicts {
		if c.Reason == "stock_insufficient" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stock_insufficient conflict, got %+v", resp.Conflicts)
	}
}

// TestPrecheck_ProductOffsale 下架商品应在 precheck 中被阻断。
func TestPrecheck_ProductOffsale(t *testing.T) {
	repo := &mockCartRepo{
		items: []CartItem{
			{ID: 3, UserID: 100, SkuID: 30, Qty: 1, SnapshotPriceCents: 800},
		},
		skus: map[int64]*CartSKUInfo{
			30: newSKUInfo(30, 3, 800, "active", "draft", 10, 0),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		3: {ID: 3, Status: "draft"},
	}}
	svc := NewService(repo, prodRepo, nil)

	resp, err := svc.Precheck(context.Background(), 100, []int64{3})
	if err != nil {
		t.Fatalf("Precheck error: %v", err)
	}
	if resp.OK {
		t.Fatal("expected OK=false")
	}
	if len(resp.Conflicts) == 0 || resp.Conflicts[0].Reason != "product_offsale" {
		t.Fatalf("expected product_offsale, got %+v", resp.Conflicts)
	}
}

// TestUpdate_RejectsQtyAboveAvailable 数量修改不能超过当前可售库存。
func TestUpdate_RejectsQtyAboveAvailable(t *testing.T) {
	repo := &mockCartRepo{
		items: []CartItem{
			{ID: 4, UserID: 100, SkuID: 40, Qty: 1, SnapshotPriceCents: 1000},
		},
		skus: map[int64]*CartSKUInfo{
			40: newSKUInfo(40, 4, 1000, "active", "onsale", 5, 2),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		4: {ID: 4, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	err := svc.Update(context.Background(), 4, 100, 4)
	if err == nil {
		t.Fatal("expected stock limit error")
	}
	ae, ok := err.(*errs.AppError)
	if !ok || ae.Code != errs.ErrParam.Code {
		t.Fatalf("expected ErrParam, got %v", err)
	}
}

func TestPrecheck_NoConflictsReturnsEmptySlice(t *testing.T) {
	repo := &mockCartRepo{
		items: []CartItem{
			{ID: 3, UserID: 100, SkuID: 30, Qty: 1, SnapshotPriceCents: 1200},
		},
		skus: map[int64]*CartSKUInfo{
			30: newSKUInfo(30, 3, 1200, "active", "onsale", 10, 0),
		},
	}
	prodRepo := &mockProductRepo{products: map[int64]product.Product{
		3: {ID: 3, Status: "onsale"},
	}}
	svc := NewService(repo, prodRepo, nil)

	resp, err := svc.Precheck(context.Background(), 100, []int64{3})
	if err != nil {
		t.Fatalf("Precheck error: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}
	if resp.Conflicts == nil {
		t.Fatal("expected conflicts to be an empty slice, got nil")
	}
	if len(resp.Conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %+v", resp.Conflicts)
	}
}

func TestList_ReturnsProductVOAndFallsBackToMainImage(t *testing.T) {
	repo := &mockCartRepo{
		details: []CartItemDetail{
			{
				ID:                   1,
				UserID:               100,
				SkuID:                10,
				Qty:                  2,
				SnapshotPriceCents:   900,
				SkuPriceCents:        1000,
				SkuStatus:            "active",
				SkuStock:             100,
				SkuLockedStock:       0,
				SkuImage:             "",
				SkuAttrs:             []byte(`["红色","L"]`),
				ProductID:            1,
				ProductCategoryID:    9,
				ProductTitle:         "测试商品",
				ProductSubtitle:      "副标题",
				ProductMainImage:     "https://cdn.example.com/main.jpg",
				ProductTags:          []byte(`["新品"]`),
				ProductStatus:        "onsale",
				ProductSales:         22,
				ProductPriceMinCents: 1000,
				ProductPriceMaxCents: 2000,
			},
		},
	}
	svc := newTestService(repo, 0)

	resp, err := svc.List(context.Background(), 100)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Product.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("expected product main image, got %s", item.Product.MainImage)
	}
	if item.SkuImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("expected sku image fallback to product main image, got %s", item.SkuImage)
	}
	if item.Product.Title != "测试商品" {
		t.Fatalf("expected product title, got %s", item.Product.Title)
	}
}