package product

import (
	"context"

	"gorm.io/gorm"
)

// ---- CategoryRepo ----

// 分类数据访问接口。
type CategoryRepo interface {
	// ListAll 查询全部未软删除的商品分类。
	ListAll(ctx context.Context) ([]*Category, error)
}

type categoryRepoImpl struct{ db *gorm.DB }

// NewCategoryRepo 构造 CategoryRepo。
func NewCategoryRepo(db *gorm.DB) CategoryRepo {
	return &categoryRepoImpl{db: db}
}

func (r *categoryRepoImpl) ListAll(ctx context.Context) ([]*Category, error) {
	var categories []*Category
	if err := r.db.WithContext(ctx).
		Order("parent_id ASC, sort ASC, id ASC").
		Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// ---- ProductRepo ----

// ProductFilter 商品列表查询条件。
type ProductFilter struct {
	CategoryID int64
	Status     string
	Keyword    string
	Sort       string // latest / popular / price_asc / price_desc / hot
	InStock    bool
	Page       int
	PageSize   int
}

// 商品数据访问接口。
type ProductRepo interface {
	// ListByIDs 按 ID 精准查询（IDs 数量受 service 层限制）。
	ListByIDs(ctx context.Context, ids []int64) ([]*Product, error)
	// Search 按条件模糊查询，返回列表和总数。
	Search(ctx context.Context, f ProductFilter) ([]*Product, int, error)
}

type productRepoImpl struct{ db *gorm.DB }

// NewProductRepo 构造 ProductRepo。
func NewProductRepo(db *gorm.DB) ProductRepo {
	return &productRepoImpl{db: db}
}

func (r *productRepoImpl) ListByIDs(ctx context.Context, ids []int64) ([]*Product, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var products []*Product
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepoImpl) Search(ctx context.Context, f ProductFilter) ([]*Product, int, error) {
	q := r.db.WithContext(ctx).Model(&Product{})

	if f.CategoryID > 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("title LIKE ? OR subtitle LIKE ?", like, like)
	}
	if f.InStock {
		q = q.Where("EXISTS (SELECT 1 FROM sku s WHERE s.product_id = product.id AND s.stock > s.locked_stock)")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页，上限与 DTO binding(max=50) 保持一致
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	// 排序
	switch f.Sort {
	case "price_asc":
		q = q.Order("price_min_cents ASC")
	case "price_desc":
		q = q.Order("price_min_cents DESC")
	case "hot", "popular":
		q = q.Order("(sales + virtual_sales) DESC, id ASC")
	case "latest":
		q = q.Order("created_at DESC")
	default:
		q = q.Order("sort ASC, id ASC")
	}

	var products []*Product
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, int(total), nil
}
