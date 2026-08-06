package product

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/singleflight"
	"shop/internal/pkg/types"
)

const (
	categoriesCacheKey = "shop:categories"
	categoryCacheTTL   = 5 * time.Minute
	hotProductKey      = "shop:hot:product"
	hotProductTTL      = 30 * time.Minute
	// hotProductPageSize 热卖商品一次全量返回的条数上限，与 DTO binding(max=50)、repo clamp 上限一致
	hotProductPageSize = 50
)

type Service struct {
	rdb             *redis.Client
	logger          *zap.Logger
	categoryRepo    CategoryRepo
	productRepo     ProductRepo
	favoriteRepo    FavoriteRepo
	viewHistoryRepo ViewHistoryRepo
}

func NewService(rdb *redis.Client, logger *zap.Logger, categoryRepo CategoryRepo, productRepo ProductRepo, favoriteRepo FavoriteRepo, viewHistoryRepo ViewHistoryRepo) *Service {
	return &Service{rdb: rdb, logger: logger, categoryRepo: categoryRepo, productRepo: productRepo, favoriteRepo: favoriteRepo, viewHistoryRepo: viewHistoryRepo}
}

func (s *Service) ListCategories(ctx context.Context) ([]CategoriesResp, error) {
	var result []CategoriesResp
	// 从redis中获取result
	dataBytes, err := s.rdb.Get(ctx, categoriesCacheKey).Bytes()
	if err == nil {
		if err := json.Unmarshal(dataBytes, &result); err == nil {
			return result, nil
		}
	}

	// redis无数据时，重新构建,使用singleflight避免缓存击穿
	categories, err := singleflight.Lock(categoriesCacheKey, func() ([]*Category, error) { return s.categoryRepo.ListAll(ctx) })
	if err != nil {
		return nil, err
	}

	result = s.buildCategoriesTree(categories)

	// 写入redis中
	dataBytes, err = json.Marshal(result)
	if err != nil {
		s.logger.Warn("categories tree 序列化失败", zap.Error(err))
	}
	err = s.rdb.Set(ctx, categoriesCacheKey, dataBytes, categoryCacheTTL).Err()
	if err != nil {
		s.logger.Warn("categories tree 写入redis失败", zap.Error(err))
	}

	return result, nil
}

func (s *Service) buildCategoriesTree(array []*Category) []CategoriesResp {

	tree := make(map[int64]*CategoriesResp, len(array))
	// 把所有分类放入 tree
	tree[0] = &CategoriesResp{}
	for _, v := range array {
		tree[v.ID] = ToCategoriesResp(v)
	}

	// 根据 ParentID 建立父子关系
	for _, v := range array {
		node := tree[v.ID]
		parentNode := tree[v.ParentID]
		if parentNode == nil {
			continue
		}
		parentNode.Children = append(parentNode.Children, *node)
	}
	return tree[0].Children

}

func (s *Service) ListProducts(ctx context.Context, req *ProductListReq) ([]ProductResp, int, error) {
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 50
	}

	IDs, err := req.IDsList()
	if err != nil {
		return nil, 0, err
	}

	var products []*Product
	var total int

	if len(IDs) == 0 {
		// 使用模糊匹配查询
		products, total, err = s.productRepo.Search(ctx, ProductFilter{
			CategoryID: int64(req.CategoryID),
			Status:     req.Status,
			Keyword:    req.Keyword,
			Sort:       req.Sort,
			InStock:    req.InStock,
			Page:       req.Page,
			PageSize:   req.PageSize,
		})
	} else {
		// 使用id精准查询
		products, err = s.productRepo.ListByIDs(ctx, IDs)
		total = len(products)
	}
	if err != nil {
		return nil, 0, err
	}

	list := make([]ProductResp, len(products))
	for i, p := range products {
		list[i] = *ToProductResp(p)
	}
	return list, total, nil
}

// ListHotProducts 热卖商品列表。
// 固定返回第一页全量数据，不接受前端分页/筛选参数，保证缓存与 singleflight 的 key 一致。
func (s *Service) ListHotProducts(ctx context.Context) ([]ProductResp, error) {
	var resp []ProductResp
	// 1.先查redis
	bytes, err := s.rdb.Get(ctx, hotProductKey).Bytes()
	if err == nil {
		jsonerr := json.Unmarshal(bytes, &resp)
		if jsonerr == nil {
			return resp, nil
		} else {
			// 移除坏的hot product list
			s.logger.Warn("hot product unmarshal failed",
				zap.String("hotProductKey", hotProductKey),
				zap.Error(jsonerr),
			)
			s.rdb.Del(ctx, hotProductKey)

		}
	}

	// 2.redis挂了，或者是product list unmarshal失败，使用singleflight避免大量请求打到数据库
	// 热卖商品固定全量（第一页）查询，key 用 hotProductKey，与 redis 缓存 key 保持一致，
	// 避免不同参数的请求被 singleflight 错误合并
	products, err := singleflight.Lock(hotProductKey, func() ([]*Product, error) {
		ps, _, e := s.productRepo.Search(ctx, ProductFilter{
			Sort:     "hot",
			Page:     1,
			PageSize: hotProductPageSize,
		})
		return ps, e
	})

	if err != nil {
		return nil, err
	}

	for _, p := range products {
		resp = append(resp, *ToProductResp(p))
	}

	// 异步写入缓存
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		b, err := json.Marshal(resp)
		if err != nil {
			s.logger.Warn("hot product marshal 失败", zap.Error(err))
			return
		}

		if err = s.rdb.Set(ctx, hotProductKey, b, hotProductTTL).Err(); err != nil {
			s.logger.Warn("redis 写入hot product失败", zap.Error(err))
			return
		}
	}()

	return resp, nil
}

// GetProduct 商品详情：商品基础信息 + 规格树 + SKU 列表 + 收藏状态。
// userID 为 0 表示未登录，直接返回 is_favorite=false。
func (s *Service) GetProduct(ctx context.Context, productID, userID int64) (*ProductDetailResp, error) {
	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	// 规格树：先查规格名，再按规格 ID 批量查规格值
	specs, err := s.productRepo.ListSpecs(ctx, productID)
	if err != nil {
		return nil, err
	}
	specIDs := make([]int64, 0, len(specs))
	for _, sp := range specs {
		specIDs = append(specIDs, sp.ID)
	}
	values, err := s.productRepo.ListSpecValues(ctx, specIDs)
	if err != nil {
		return nil, err
	}
	// 按 spec_id 分组，方便挂到对应规格名下
	valueMap := make(map[int64][]SpecValueResp, len(values))
	for _, v := range values {
		valueMap[v.SpecID] = append(valueMap[v.SpecID], SpecValueResp{
			ID:    types.Int64Str(v.ID),
			Value: v.Value,
			Sort:  v.Sort,
		})
	}
	specResp := make([]SpecResp, 0, len(specs))
	for _, sp := range specs {
		specResp = append(specResp, SpecResp{
			ID:     types.Int64Str(sp.ID),
			Name:   sp.Name,
			Sort:   sp.Sort,
			Values: valueMap[sp.ID],
		})
	}

	// SKU 列表
	skus, err := s.productRepo.ListSKUs(ctx, productID)
	if err != nil {
		return nil, err
	}
	skuResp := make([]UserSKUResp, 0, len(skus))
	for _, sku := range skus {
		skuResp = append(skuResp, toUserSKUResp(sku))
	}

	// 收藏状态 + 浏览历史：未登录直接跳过，登录才查
	isFavorite := false
	if userID > 0 {
		isFavorite, err = s.favoriteRepo.IsFavorite(ctx, userID, productID)
		if err != nil {
			return nil, err
		}
		// 记录浏览历史：非核心路径，失败只记日志，不阻断详情返回
		if err := s.viewHistoryRepo.Upsert(ctx, userID, productID); err != nil {
			s.logger.Warn("record view history failed",
				zap.Int64("user_id", userID),
				zap.Int64("product_id", productID),
				zap.Error(err))
		}
	}

	base := ToProductResp(product)
	return &ProductDetailResp{
		ProductResp: *base,
		Images:      json.RawMessage(product.Images),
		VideoURL:    product.VideoURL,
		DetailHTML:  product.DetailHTML,
		DetailNodes: json.RawMessage(product.DetailNodes),
		Specs:       specResp,
		SKUs:        skuResp,
		IsFavorite:  isFavorite,
	}, nil
}

// AddFavorite 收藏商品。
func (s *Service) AddFavorite(ctx context.Context, userID, productID int64) error {
	// 校验商品存在，避免收藏指向不存在的商品
	if _, err := s.productRepo.GetByID(ctx, productID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return err
	}
	return s.favoriteRepo.Add(ctx, userID, productID)
}

// RemoveFavorite 取消收藏。
func (s *Service) RemoveFavorite(ctx context.Context, userID, productID int64) error {
	return s.favoriteRepo.Remove(ctx, userID, productID)
}

// ListFavorites 用户收藏的商品列表。
func (s *Service) ListFavorites(ctx context.Context, userID int64, page, pageSize int) ([]ProductResp, int, error) {
	products, total, err := s.favoriteRepo.List(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return toProductRespList(products), total, nil
}

// GetViewHistory 用户最近浏览的商品列表。
func (s *Service) GetViewHistory(ctx context.Context, userID int64, page, pageSize int) ([]ProductResp, int, error) {
	products, total, err := s.viewHistoryRepo.List(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return toProductRespList(products), total, nil
}

// toProductRespList 批量转换 Product 实体为响应 DTO。
func toProductRespList(products []*Product) []ProductResp {
	list := make([]ProductResp, 0, len(products))
	for _, p := range products {
		list = append(list, *ToProductResp(p))
	}
	return list
}

// ---- 后台管理 ----

// AdminListCategories 后台分类列表（树形）。
func (s *Service) AdminListCategories(ctx context.Context) ([]CategoriesResp, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return s.buildCategoriesTree(categories), nil
}

// AdminCreateCategory 创建分类。
func (s *Service) AdminCreateCategory(ctx context.Context, req *AdminCategoryReq) (*CategoriesResp, error) {
	c := &Category{
		ParentID: int64(req.ParentID),
		Name:     req.Name,
		Icon:     req.Icon,
		Sort:     req.Sort,
		Status:   req.Status,
	}
	if c.Status == "" {
		c.Status = "enabled"
	}
	if err := s.categoryRepo.Create(ctx, c); err != nil {
		return nil, err
	}
	return ToCategoriesResp(c), nil
}

// AdminUpdateCategory 更新分类。
func (s *Service) AdminUpdateCategory(ctx context.Context, id int64, req *AdminCategoryReq) (*CategoriesResp, error) {
	c := &Category{
		ID:       id,
		ParentID: int64(req.ParentID),
		Name:     req.Name,
		Icon:     req.Icon,
		Sort:     req.Sort,
		Status:   req.Status,
	}
	if err := s.categoryRepo.Update(ctx, c); err != nil {
		return nil, err
	}
	return ToCategoriesResp(c), nil
}

// AdminDeleteCategory 删除分类。
func (s *Service) AdminDeleteCategory(ctx context.Context, id int64) error {
	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return err
	}
	return nil
}

// AdminListProducts 后台商品列表。
func (s *Service) AdminListProducts(ctx context.Context, req *ProductListReq) ([]AdminProductResp, int, error) {
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 50
	}
	products, total, err := s.productRepo.Search(ctx, ProductFilter{
		CategoryID: int64(req.CategoryID),
		Status:     req.Status,
		Keyword:    req.Keyword,
		Sort:       req.Sort,
		InStock:    req.InStock,
		Page:       req.Page,
		PageSize:   req.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	list := make([]AdminProductResp, 0, len(products))
	for _, p := range products {
		list = append(list, *toAdminProductResp(p))
	}
	return list, total, nil
}

// AdminGetProduct 后台商品详情。
func (s *Service) AdminGetProduct(ctx context.Context, id int64) (*AdminProductDetailResp, error) {
	p, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return s.buildAdminDetail(ctx, p)
}

// AdminCreateProduct 创建商品（含规格树 + SKU，事务原子）。
func (s *Service) AdminCreateProduct(ctx context.Context, req *AdminProductReq) (*AdminProductDetailResp, error) {
	p, specs, skus, err := s.buildProduct(req)
	if err != nil {
		return nil, err
	}
	if err := s.productRepo.CreateWithDetails(ctx, p, specs, skus); err != nil {
		return nil, err
	}
	return s.buildAdminDetail(ctx, p)
}

// AdminUpdateProduct 更新商品：更新基础字段 + 重建规格树与 SKU（先删后插）。
func (s *Service) AdminUpdateProduct(ctx context.Context, id int64, req *AdminProductReq) (*AdminProductDetailResp, error) {
	old, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	p, specs, skus, err := s.buildProduct(req)
	if err != nil {
		return nil, err
	}
	// 保留不可由前端修改的字段
	p.ID = id
	p.Status = old.Status
	p.Sales = old.Sales
	p.VirtualSales = old.VirtualSales
	p.OnSaleAt = old.OnSaleAt
	if err := s.productRepo.UpdateBase(ctx, p); err != nil {
		return nil, err
	}
	if err := s.productRepo.ReplaceDetails(ctx, id, specs, skus); err != nil {
		return nil, err
	}
	return s.buildAdminDetail(ctx, p)
}

// AdminDeleteProduct 删除商品（软删除）。
func (s *Service) AdminDeleteProduct(ctx context.Context, id int64) error {
	if err := s.productRepo.DeleteByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return err
	}
	return nil
}

// AdminCopyProduct 复制商品：深拷贝基础信息 + 规格树 + SKU，新商品状态为草稿。
func (s *Service) AdminCopyProduct(ctx context.Context, id int64) (*AdminProductDetailResp, error) {
	old, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	// 复制基础信息（重置 ID、销量、上架时间，状态回草稿）
	p := *old
	p.ID = 0
	p.Title = old.Title + "(副本)"
	p.Status = "draft"
	p.Sales = 0
	p.VirtualSales = 0
	p.OnSaleAt = nil

	// 复制规格树
	oldSpecs, err := s.productRepo.ListSpecs(ctx, id)
	if err != nil {
		return nil, err
	}
	specIDs := make([]int64, 0, len(oldSpecs))
	for _, sp := range oldSpecs {
		specIDs = append(specIDs, sp.ID)
	}
	oldValues, err := s.productRepo.ListSpecValues(ctx, specIDs)
	if err != nil {
		return nil, err
	}
	valueMap := make(map[int64][]*ProductSpecValue)
	for _, v := range oldValues {
		valueMap[v.SpecID] = append(valueMap[v.SpecID], v)
	}
	specs := make([]*ProductSpec, 0, len(oldSpecs))
	for _, sp := range oldSpecs {
		nsp := &ProductSpec{Name: sp.Name, Sort: sp.Sort}
		for _, v := range valueMap[sp.ID] {
			nsp.Values = append(nsp.Values, &ProductSpecValue{Value: v.Value, Sort: v.Sort})
		}
		specs = append(specs, nsp)
	}

	// 复制 SKU
	oldSKUs, err := s.productRepo.ListSKUs(ctx, id)
	if err != nil {
		return nil, err
	}
	skus := make([]*SKU, 0, len(oldSKUs))
	for _, sku := range oldSKUs {
		nsku := *sku
		nsku.ID = 0
		nsku.ProductID = 0
		skus = append(skus, &nsku)
	}

	if err := s.productRepo.CreateWithDetails(ctx, &p, specs, skus); err != nil {
		return nil, err
	}
	return s.buildAdminDetail(ctx, &p)
}

// AdminOnSale 上架商品。
func (s *Service) AdminOnSale(ctx context.Context, id int64) error {
	if err := s.ensureProductExists(ctx, id); err != nil {
		return err
	}
	now := time.Now()
	return s.productRepo.UpdateStatus(ctx, []int64{id}, "onsale", &now)
}

// AdminOffSale 下架商品。
func (s *Service) AdminOffSale(ctx context.Context, id int64) error {
	if err := s.ensureProductExists(ctx, id); err != nil {
		return err
	}
	return s.productRepo.UpdateStatus(ctx, []int64{id}, "offsale", nil)
}

// AdminBatchStatus 批量设置商品状态。
func (s *Service) AdminBatchStatus(ctx context.Context, req *AdminBatchStatusReq) error {
	var onSaleAt *time.Time
	if req.Status == "onsale" {
		now := time.Now()
		onSaleAt = &now
	}
	return s.productRepo.UpdateStatus(ctx, req.IDs, req.Status, onSaleAt)
}

// AdminBatchPrice 批量更新 SKU 价格。
func (s *Service) AdminBatchPrice(ctx context.Context, req *AdminBatchPriceReq) error {
	priceMap := make(map[int64]int64, len(req.Items))
	for _, item := range req.Items {
		priceMap[item.SKUID] = item.PriceCents
	}
	return s.productRepo.UpdateSKUPrices(ctx, priceMap)
}

// buildProduct 将后台商品请求转换为实体（含规格树、SKU），并计算价格区间。
// 创建时状态默认 draft。
func (s *Service) buildProduct(req *AdminProductReq) (*Product, []*ProductSpec, []*SKU, error) {
	p := &Product{
		CategoryID:        int64(req.CategoryID),
		Title:             req.Title,
		Subtitle:          req.Subtitle,
		MainImage:         req.MainImage,
		Images:            jraw(req.Images, "[]"),
		VideoURL:          req.VideoURL,
		DetailHTML:        req.DetailHTML,
		DetailNodes:       jraw(req.DetailNodes, "{}"),
		Unit:              req.Unit,
		IsVirtual:         req.IsVirtual,
		FreightTemplateID: req.FreightTemplateID,
		Sort:              req.Sort,
		Tags:              jraw(req.Tags, "[]"),
		Status:            "draft",
	}

	specs := make([]*ProductSpec, 0, len(req.Specs))
	for _, sp := range req.Specs {
		spec := &ProductSpec{Name: sp.Name, Sort: sp.Sort}
		for _, v := range sp.Values {
			spec.Values = append(spec.Values, &ProductSpecValue{Value: v})
		}
		specs = append(specs, spec)
	}

	skus := make([]*SKU, 0, len(req.SKUs))
	var minCents, maxCents int64
	for i, in := range req.SKUs {
		attrs, err := json.Marshal(in.Attrs)
		if err != nil {
			return nil, nil, nil, err
		}
		skus = append(skus, &SKU{
			Attrs:              JSON(attrs),
			PriceCents:         in.PriceCents,
			OriginalPriceCents: in.OriginalPriceCents,
			Stock:              in.Stock,
			WeightG:            in.WeightG,
			SkuCode:            in.SkuCode,
			Barcode:            in.Barcode,
			Image:              in.Image,
			Status:             in.Status,
			LowStockThreshold:  in.LowStockThreshold,
		})
		if i == 0 {
			minCents, maxCents = in.PriceCents, in.PriceCents
		} else {
			if in.PriceCents < minCents {
				minCents = in.PriceCents
			}
			if in.PriceCents > maxCents {
				maxCents = in.PriceCents
			}
		}
	}
	p.PriceMinCents = minCents
	p.PriceMaxCents = maxCents
	return p, specs, skus, nil
}

// buildAdminDetail 组装后台商品详情响应。
func (s *Service) buildAdminDetail(ctx context.Context, p *Product) (*AdminProductDetailResp, error) {
	specs, err := s.productRepo.ListSpecs(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	specIDs := make([]int64, 0, len(specs))
	for _, sp := range specs {
		specIDs = append(specIDs, sp.ID)
	}
	values, err := s.productRepo.ListSpecValues(ctx, specIDs)
	if err != nil {
		return nil, err
	}
	valueMap := make(map[int64][]SpecValueResp)
	for _, v := range values {
		valueMap[v.SpecID] = append(valueMap[v.SpecID], SpecValueResp{
			ID:    types.Int64Str(v.ID),
			Value: v.Value,
			Sort:  v.Sort,
		})
	}
	specResp := make([]SpecResp, 0, len(specs))
	for _, sp := range specs {
		specResp = append(specResp, SpecResp{
			ID:     types.Int64Str(sp.ID),
			Name:   sp.Name,
			Sort:   sp.Sort,
			Values: valueMap[sp.ID],
		})
	}

	skus, err := s.productRepo.ListSKUs(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	skuResp := make([]AdminSKUResp, 0, len(skus))
	for _, sku := range skus {
		skuResp = append(skuResp, toAdminSKUResp(sku))
	}

	base := ToProductResp(p)
	return &AdminProductDetailResp{
		ProductResp:  *base,
		VirtualSales: p.VirtualSales,
		Images:       json.RawMessage(p.Images),
		VideoURL:     p.VideoURL,
		DetailHTML:   p.DetailHTML,
		DetailNodes:  json.RawMessage(p.DetailNodes),
		Specs:        specResp,
		SKUs:         skuResp,
	}, nil
}

// ensureProductExists 校验商品存在，不存在返回 ErrNotFound。
func (s *Service) ensureProductExists(ctx context.Context, id int64) error {
	if _, err := s.productRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return err
	}
	return nil
}

// jraw 将 json.RawMessage 转为 JSON，为空时用默认值兜底（避免写入 NULL）。
func jraw(raw json.RawMessage, def string) JSON {
	if len(raw) == 0 {
		return JSON(def)
	}
	return JSON(raw)
}
