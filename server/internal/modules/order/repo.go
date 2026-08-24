package order

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// OrderRepo 订单数据访问接口。
type OrderRepo interface {
	// InsertOrder 事务内插入订单主表 + 明细表。
	InsertOrder(ctx context.Context, o *Order, items []*OrderItem) error
	// FindSKUsByIDs 批量按 SKU ID 查 SKU + Product 信息,返回以 skuID 为键的 map。
	// 不存在的 skuID 不会出现在返回 map 里（调用方按 skuID 取值要做 missing 检查）。
	FindSKUsByIDs(ctx context.Context, skuIDs []int64) (map[int64]*OrderSKUInfo, error)
	// FindFreightTemplatesBySKUIDs 获取 SKU 对应商品的运费模板元数据；规则按地址按需查询。
	FindFreightTemplatesBySKUIDs(ctx context.Context, skuIDs []int64) ([]*FreightTemplate, error)
	// FindBestRuleForRegion 查询模板在指定地区下优先级最高的一条规则。
	FindBestRuleForRegion(ctx context.Context, templateID int64, regionCodes []string) (*FreightTemplateRule, error)
	// GetByID 按主键查订单。
	GetByID(ctx context.Context, id int64) (*Order, error)
	// ListByUserID 分页查用户的订单。
	ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*Order, int, error)
	// UpdateStatus 更新订单状态。
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// FreightThresholdRepo 整单免运阈值数据访问接口。
type FreightThresholdRepo interface {
	// GetByRemote 根据"是否偏远"取整单免运阈值。
	// 找不到记录(未配置)时返回 (nil, nil),调用方按"未启用"处理(继续按 product 算运费)。
	GetByRemote(ctx context.Context, isRemote bool) (*FreightOrderThreshold, error)
}

// orderRepoImpl OrderRepo 的 gorm 实现。
type orderRepoImpl struct{ db *gorm.DB }

// NewOrderRepo 构造 OrderRepo。
func NewOrderRepo(db *gorm.DB) OrderRepo {
	return &orderRepoImpl{db: db}
}

// thresholdRepoImpl FreightThresholdRepo 的 gorm 实现。
type thresholdRepoImpl struct{ db *gorm.DB }

// NewFreightThresholdRepo 构造 FreightThresholdRepo。
func NewFreightThresholdRepo(db *gorm.DB) FreightThresholdRepo {
	return &thresholdRepoImpl{db: db}
}

// GetByRemote 按 is_remote 取阈值。gorm.ErrRecordNotFound 转成 (nil, nil),不报错。
func (r *thresholdRepoImpl) GetByRemote(ctx context.Context, isRemote bool) (*FreightOrderThreshold, error) {
	var t FreightOrderThreshold
	err := r.db.WithContext(ctx).Where("is_remote = ?", isRemote).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
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
			s.weight_g       AS weight_g,
			s.attrs           AS sku_attrs,
			p.id              AS product_id,
			p.title           AS product_title,
			p.main_image      AS product_main_image,
			p.freight_template_id AS freight_template_id,
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

// FindFreightTemplatesBySKUIDs 根据 SKU 关联的商品模板 ID 批量加载模板元数据。
// 规则和地区按地址按需查询，不在这里 Preload。
func (r *orderRepoImpl) FindFreightTemplatesBySKUIDs(ctx context.Context, skuIDs []int64) ([]*FreightTemplate, error) {
	if len(skuIDs) == 0 {
		return []*FreightTemplate{}, nil
	}

	var templates []FreightTemplate
	err := r.db.WithContext(ctx).
		Table("freight_template AS t").
		Select("DISTINCT t.*").
		Joins("JOIN product AS p ON p.freight_template_id = t.id").
		Joins("JOIN sku AS s ON s.product_id = p.id").
		Where("s.id IN ?", skuIDs).
		Find(&templates).Error
	if err != nil {
		return nil, err
	}
	out := make([]*FreightTemplate, 0, len(templates))
	for i := range templates {
		out = append(out, &templates[i])
	}
	return out, nil
}

// FindBestRuleForRegion 按区、市、省和默认规则的优先级查询一条最佳规则。
func (r *orderRepoImpl) FindBestRuleForRegion(ctx context.Context, templateID int64, regionCodes []string) (*FreightTemplateRule, error) {
	if templateID <= 0 || len(regionCodes) == 0 {
		return nil, nil
	}
	var rule FreightTemplateRule
	err := r.db.WithContext(ctx).Raw(`
		SELECT r.*
		FROM freight_template_rule AS r
		LEFT JOIN freight_rule_region AS region ON region.rule_id = r.id
		WHERE r.template_id = ?
		  AND (region.region_code IN ? OR region.rule_id IS NULL)
		ORDER BY
			CASE region.level
				WHEN 'district' THEN 3
				WHEN 'city' THEN 2
				WHEN 'province' THEN 1
				ELSE 0
			END DESC,
			r.priority DESC,
			r.id ASC
		LIMIT 1
	`, templateID, regionCodes).Scan(&rule).Error
	if err != nil {
		return nil, err
	}
	if rule.ID == 0 {
		return nil, nil
	}
	return &rule, nil
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
