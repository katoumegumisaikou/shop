package product

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	categoriesCacheKey = "shop:categories"
	categoryCacheTTL   = 5 * time.Minute
	hotProductKey      = "shop:hot:product"
	hotProductTTL      = 30 * time.Minute
)

type Service struct {
	rdb          *redis.Client
	logger       *zap.Logger
	categoryRepo CategoryRepo
	productRepo  ProductRepo
}

func NewService(rdb *redis.Client, logger *zap.Logger, categoryRepo CategoryRepo, productRepo ProductRepo) *Service {
	return &Service{rdb: rdb, logger: logger, categoryRepo: categoryRepo, productRepo: productRepo}
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

	// redis无数据时，重新构建
	categories, err := s.categoryRepo.ListAll(ctx)
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

func (s *Service) ListHotProducts(ctx context.Context, req ProductListReq) ([]ProductResp, error) {
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

	// 2.redis挂了，或者是product list unmarshal失败
	products, _, err := s.productRepo.Search(ctx, ProductFilter{
		Status:   req.Status,
		Sort:     "hot",
		InStock:  req.InStock,
		Page:     req.Page,
		PageSize: req.PageSize,
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
