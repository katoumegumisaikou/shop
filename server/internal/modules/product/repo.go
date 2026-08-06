package product

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// GetByID 按主键查询单个商品（自动过滤软删除）。
	GetByID(ctx context.Context, id int64) (*Product, error)
	// ListSpecs 查询商品的全部规格名。
	ListSpecs(ctx context.Context, productID int64) ([]*ProductSpec, error)
	// ListSpecValues 按规格 ID 批量查询规格值。
	ListSpecValues(ctx context.Context, specIDs []int64) ([]*ProductSpecValue, error)
	// ListSKUs 查询商品的全部 SKU。
	ListSKUs(ctx context.Context, productID int64) ([]*SKU, error)
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

func (r *productRepoImpl) GetByID(ctx context.Context, id int64) (*Product, error) {
	var p Product
	// First 自动附加 deleted_at IS NULL，找不到时返回 gorm.ErrRecordNotFound
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *productRepoImpl) ListSpecs(ctx context.Context, productID int64) ([]*ProductSpec, error) {
	var specs []*ProductSpec
	if err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("sort ASC, id ASC").
		Find(&specs).Error; err != nil {
		return nil, err
	}
	return specs, nil
}

func (r *productRepoImpl) ListSpecValues(ctx context.Context, specIDs []int64) ([]*ProductSpecValue, error) {
	if len(specIDs) == 0 {
		return nil, nil
	}
	var values []*ProductSpecValue
	if err := r.db.WithContext(ctx).
		Where("spec_id IN ?", specIDs).
		Order("sort ASC, id ASC").
		Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (r *productRepoImpl) ListSKUs(ctx context.Context, productID int64) ([]*SKU, error) {
	var skus []*SKU
	if err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		Order("id ASC").
		Find(&skus).Error; err != nil {
		return nil, err
	}
	return skus, nil
}

// ---- FavoriteRepo ----

// FavoriteRepo 用户收藏数据访问接口。
type FavoriteRepo interface {
	// IsFavorite 判断用户是否已收藏指定商品。
	IsFavorite(ctx context.Context, userID, productID int64) (bool, error)
	// Add 添加收藏（重复收藏幂等，ON CONFLICT DO NOTHING）。
	Add(ctx context.Context, userID, productID int64) error
	// Remove 取消收藏（不存在也返回 nil，幂等）。
	Remove(ctx context.Context, userID, productID int64) error
	// List 分页查询用户收藏的商品列表（按收藏时间倒序）。
	List(ctx context.Context, userID int64, page, pageSize int) ([]*Product, int, error)
}

type favoriteRepoImpl struct{ db *gorm.DB }

// NewFavoriteRepo 构造 FavoriteRepo。
func NewFavoriteRepo(db *gorm.DB) FavoriteRepo {
	return &favoriteRepoImpl{db: db}
}

func (r *favoriteRepoImpl) IsFavorite(ctx context.Context, userID, productID int64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&UserFavorite{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *favoriteRepoImpl) Add(ctx context.Context, userID, productID int64) error {
	// ON CONFLICT DO NOTHING：重复收藏不报错，幂等
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&UserFavorite{UserID: userID, ProductID: productID}).Error
}

func (r *favoriteRepoImpl) Remove(ctx context.Context, userID, productID int64) error {
	// 删除不存在的记录影响行数为 0，同样返回 nil，幂等
	return r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&UserFavorite{}).Error
}

func (r *favoriteRepoImpl) List(ctx context.Context, userID int64, page, pageSize int) ([]*Product, int, error) {
	// 分页 clamp，与 Search 保持一致
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	q := r.db.WithContext(ctx).Model(&Product{}).
		Joins("JOIN user_favorite f ON f.product_id = product.id").
		Where("f.user_id = ?", userID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var products []*Product
	if err := q.Order("f.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, int(total), nil
}

// ---- ViewHistoryRepo ----

// ViewHistoryRepo 用户浏览历史数据访问接口。
type ViewHistoryRepo interface {
	// Upsert 记录一次浏览：已存在则刷新 ViewedAt，不存在则插入。
	// 利用 (user_id, product_id) 联合主键做 ON CONFLICT，最近浏览按 ViewedAt 倒序取即可。
	Upsert(ctx context.Context, userID, productID int64) error
	// List 分页查询用户的最近浏览商品列表（按 viewed_at 倒序）。
	List(ctx context.Context, userID int64, page, pageSize int) ([]*Product, int, error)
}

type viewHistoryRepoImpl struct{ db *gorm.DB }

// NewViewHistoryRepo 构造 ViewHistoryRepo。
func NewViewHistoryRepo(db *gorm.DB) ViewHistoryRepo {
	return &viewHistoryRepoImpl{db: db}
}

func (r *viewHistoryRepoImpl) Upsert(ctx context.Context, userID, productID int64) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "product_id"}},
			DoUpdates: clause.Assignments(map[string]any{"viewed_at": time.Now()}),
		}).
		Create(&UserViewHistory{
			UserID:    userID,
			ProductID: productID,
			ViewedAt:  time.Now(),
		}).Error
}

func (r *viewHistoryRepoImpl) List(ctx context.Context, userID int64, page, pageSize int) ([]*Product, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	q := r.db.WithContext(ctx).Model(&Product{}).
		Joins("JOIN user_view_history v ON v.product_id = product.id").
		Where("v.user_id = ?", userID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var products []*Product
	if err := q.Order("v.viewed_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, int(total), nil
}
