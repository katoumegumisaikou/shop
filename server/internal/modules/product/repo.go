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
	// Create 创建分类。
	Create(ctx context.Context, c *Category) error
	// Update 更新分类指定字段（parent_id/name/icon/sort/status）。
	Update(ctx context.Context, c *Category) error
	// Delete 软删除分类。
	Delete(ctx context.Context, id int64) error
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

func (r *categoryRepoImpl) Create(ctx context.Context, c *Category) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *categoryRepoImpl) Update(ctx context.Context, c *Category) error {
	return r.db.WithContext(ctx).Model(c).Updates(map[string]any{
		"parent_id": c.ParentID,
		"name":      c.Name,
		"icon":      c.Icon,
		"sort":      c.Sort,
		"status":    c.Status,
	}).Error
}

func (r *categoryRepoImpl) Delete(ctx context.Context, id int64) error {
	var c Category
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&c).Error
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
	// CreateWithDetails 事务内创建商品 + 规格树 + SKU。
	CreateWithDetails(ctx context.Context, p *Product, specs []*ProductSpec, skus []*SKU) error
	// UpdateBase 更新商品基础字段。
	UpdateBase(ctx context.Context, p *Product) error
	// DeleteByID 软删除商品。
	DeleteByID(ctx context.Context, id int64) error
	// UpdateStatus 批量更新商品状态（draft/onsale/offsale）。
	UpdateStatus(ctx context.Context, ids []int64, status string, onSaleAt *time.Time) error
	// ReplaceDetails 事务内重建商品规格树与 SKU（先删后插）。
	ReplaceDetails(ctx context.Context, productID int64, specs []*ProductSpec, skus []*SKU) error
	// UpdateSKUPrices 批量更新 SKU 价格。
	UpdateSKUPrices(ctx context.Context, priceMap map[int64]int64) error
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

// createSpecs 在事务内创建规格树，并把每个规格的规格值挂上对应 SpecID。
func createSpecs(tx *gorm.DB, specs []*ProductSpec) error {
	for _, sp := range specs {
		if err := tx.Create(sp).Error; err != nil {
			return err
		}
		for _, v := range sp.Values {
			v.SpecID = sp.ID
		}
		if len(sp.Values) > 0 {
			if err := tx.Create(&sp.Values).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *productRepoImpl) CreateWithDetails(ctx context.Context, p *Product, specs []*ProductSpec, skus []*SKU) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		if err := createSpecs(tx, specs); err != nil {
			return err
		}
		for _, sku := range skus {
			sku.ProductID = p.ID
		}
		if len(skus) > 0 {
			return tx.Create(&skus).Error
		}
		return nil
	})
}

func (r *productRepoImpl) UpdateBase(ctx context.Context, p *Product) error {
	return r.db.WithContext(ctx).Model(p).Updates(map[string]any{
		"category_id":         p.CategoryID,
		"title":               p.Title,
		"subtitle":            p.Subtitle,
		"main_image":          p.MainImage,
		"images":              p.Images,
		"video_url":           p.VideoURL,
		"detail_html":         p.DetailHTML,
		"detail_nodes":        p.DetailNodes,
		"unit":                p.Unit,
		"is_virtual":          p.IsVirtual,
		"freight_template_id": p.FreightTemplateID,
		"sort":                p.Sort,
		"tags":                p.Tags,
		"price_min_cents":     p.PriceMinCents,
		"price_max_cents":     p.PriceMaxCents,
	}).Error
}

func (r *productRepoImpl) DeleteByID(ctx context.Context, id int64) error {
	var p Product
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&p).Error
}

func (r *productRepoImpl) UpdateStatus(ctx context.Context, ids []int64, status string, onSaleAt *time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&Product{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"status":     status,
			"on_sale_at": onSaleAt,
		}).Error
}

func (r *productRepoImpl) ReplaceDetails(ctx context.Context, productID int64, specs []*ProductSpec, skus []*SKU) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删旧的规格值 / 规格 / SKU（这三个表无软删除，物理删除）
		var specIDs []int64
		if err := tx.Model(&ProductSpec{}).
			Where("product_id = ?", productID).
			Pluck("id", &specIDs).Error; err != nil {
			return err
		}
		if len(specIDs) > 0 {
			if err := tx.Where("spec_id IN ?", specIDs).
				Delete(&ProductSpecValue{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("product_id = ?", productID).
			Delete(&ProductSpec{}).Error; err != nil {
			return err
		}
		if err := tx.Where("product_id = ?", productID).
			Delete(&SKU{}).Error; err != nil {
			return err
		}
		// 再重建
		if err := createSpecs(tx, specs); err != nil {
			return err
		}
		for _, sku := range skus {
			sku.ProductID = productID
		}
		if len(skus) > 0 {
			return tx.Create(&skus).Error
		}
		return nil
	})
}

func (r *productRepoImpl) UpdateSKUPrices(ctx context.Context, priceMap map[int64]int64) error {
	if len(priceMap) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先确定受影响商品（改价的 SKU 属于哪些 product）
		skuIDs := make([]int64, 0, len(priceMap))
		for id := range priceMap {
			skuIDs = append(skuIDs, id)
		}
		var productIDs []int64
		if err := tx.Model(&SKU{}).
			Where("id IN ?", skuIDs).
			Distinct().
			Pluck("product_id", &productIDs).Error; err != nil {
			return err
		}

		// 更新价格
		for id, price := range priceMap {
			if err := tx.Model(&SKU{}).Where("id = ?", id).
				Update("price_cents", price).Error; err != nil {
				return err
			}
		}

		// 重算受影响商品的 price_min/max（仅统计 active 状态的 SKU）
		if len(productIDs) > 0 {
			if err := tx.Exec(`
				UPDATE product SET
					price_min_cents = COALESCE((
						SELECT MIN(price_cents) FROM sku s
						WHERE s.product_id = product.id AND s.status = 'active'), 0),
					price_max_cents = COALESCE((
						SELECT MAX(price_cents) FROM sku s
						WHERE s.product_id = product.id AND s.status = 'active'), 0)
				WHERE id IN ?`, productIDs).Error; err != nil {
				return err
			}
		}
		return nil
	})
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
