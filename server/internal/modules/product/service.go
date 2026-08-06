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
