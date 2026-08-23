package order

import (
	"context"

	"gorm.io/gorm"
)

// orderRepoImpl OrderRepo 的 gorm 实现。
type orderRepoImpl struct{ db *gorm.DB }

// NewOrderRepo 构造 OrderRepo。
func NewOrderRepo(db *gorm.DB) OrderRepo {
	return &orderRepoImpl{db: db}
}

// InsertOrder 事务内插入主表 + 明细表。
func (r *orderRepoImpl) InsertOrder(ctx context.Context, o *Order, items []*OrderItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(o).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

// FindSKUsByIDs 批量按 SKU ID 查 SKU + Product（单条 SQL JOIN）。
//
// 复用 cart.FindSKUWithProduct 的 SQL 模板,只是把单条 WHERE 改成 IN (?,?,...) 一次拉。
// 不存在的 skuID 不会出现在返回 map 里（调用方按 skuID 取值要做 missing 检查）。
func (r *orderRepoImpl) FindSKUsByIDs(ctx context.Context, skuIDs []int64) (map[int64]*OrderSKUInfo, error) {
	out := make(map[int64]*OrderSKUInfo, len(skuIDs))
	if len(skuIDs) == 0 {
		return out, nil
	}

	var rows []OrderSKUInfo
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			s.id              AS sku_id,
			s.price_cents     AS sku_price_cents,
			s.status          AS sku_status,
			s.stock           AS sku_stock,
			s.locked_stock    AS sku_locked_stock,
			s.attrs           AS sku_attrs,
			p.id              AS product_id,
			p.title           AS product_title,
			p.main_image      AS product_main_image,
			p.status          AS product_status,
			(p.deleted_at IS NOT NULL) AS product_deleted
		FROM sku s
		JOIN product p ON p.id = s.product_id
		WHERE s.id IN ?
	`, skuIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].SkuID] = &rows[i]
	}
	return out, nil
}

// GetByID 按主键查订单。
func (r *orderRepoImpl) GetByID(ctx context.Context, id int64) (*Order, error) {
	var o Order
	if err := r.db.WithContext(ctx).First(&o, id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

// ListByUserID 分页查用户的订单。
func (r *orderRepoImpl) ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*Order, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	var (
		list  []*Order
		total int64
	)
	if err := r.db.WithContext(ctx).Model(&Order{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, int(total), nil
}

// UpdateStatus 更新订单状态。
func (r *orderRepoImpl) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&Order{}).
		Where("id = ?", id).
		Update("status", status).Error
}