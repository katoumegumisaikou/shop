package order

import (
	"encoding/json"
	"fmt"
	"time"

	"database/sql/driver"
)

// ---- 自定义 JSONB 类型 ----

// AddressSnapshot 地址快照（下单时固化，不随地址变更）。
type AddressSnapshot struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Province     string `json:"province"`
	ProvinceCode string `json:"province_code"`
	City         string `json:"city"`
	CityCode     string `json:"city_code"`
	District     string `json:"district"`
	DistrictCode string `json:"district_code"`
	Street       string `json:"street,omitempty"`
	StreetCode   string `json:"street_code,omitempty"`
	Detail       string `json:"detail"`
}

func (a AddressSnapshot) Value() (driver.Value, error) {
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *AddressSnapshot) Scan(value any) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("AddressSnapshot: unsupported type %T", value)
	}
	return json.Unmarshal(b, a)
}

// RawJSON 通用 JSONB 字段。
type RawJSON []byte

func (j RawJSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "null", nil
	}
	return string(j), nil
}

func (j *RawJSON) Scan(value any) error {
	if value == nil {
		*j = RawJSON("null")
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[0:0], v...)
	case string:
		*j = RawJSON(v)
	default:
		return fmt.Errorf("RawJSON: unsupported type %T", value)
	}
	return nil
}

func (j RawJSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

func (j *RawJSON) UnmarshalJSON(data []byte) error {
	*j = append((*j)[0:0], data...)
	return nil
}

// Order 订单主表。
type Order struct {
	ID      int64  `gorm:"primaryKey"`
	OrderNo string `gorm:"size:32;not null;uniqueIndex"`
	ShopID  int64  `gorm:"not null;default:1"`
	UserID  int64  `gorm:"not null"`
	// Status 订单状态：
	//   pending 待支付
	//   paid 已支付
	//   shipped 已发货
	//   completed 已完成
	//   cancelled 已取消
	//   refunding 退款中
	//   refunded 已退款
	Status                string          `gorm:"size:16;not null"`
	GoodsCents            int64           `gorm:"not null"`
	FreightCents          int64           `gorm:"not null;default:0"`
	DiscountCents         int64           `gorm:"not null;default:0"`
	CouponDiscountCents   int64           `gorm:"not null;default:0"`
	TotalCents            int64           `gorm:"not null"`
	PayCents              int64           `gorm:"not null"`
	BalancePayCents       int64           `gorm:"column:balance_pay_cents;not null;default:0"`
	AddressSnapshot       AddressSnapshot `gorm:"type:jsonb;not null"`
	BuyerRemark           *string         `gorm:"size:200"`
	CancelRequestPending  bool            `gorm:"not null;default:false"`
	CancelRequestReason   *string         `gorm:"size:200"`
	CancelRequestAt       *time.Time
	CancelReason          *string `gorm:"size:200"`
	DistributorID         *int64
	DistributionPath      RawJSON `gorm:"type:jsonb"`
	GroupBuyOrderID       *int64
	CouponID              *int64
	PointUsed             int64 `gorm:"not null;default:0"`
	PointDeductCents      int64 `gorm:"not null;default:0"`
	FromShareUserID       *int64
	FromChannelCodeID     *int64
	IdempotencyKey        *string `gorm:"size:64"`
	CurrentPrepayID       *string `gorm:"size:64"`
	CurrentPrepayExpireAt *time.Time
	ExpireAt              time.Time `gorm:"not null"`
	PaidAt                *time.Time
	ShippedAt             *time.Time
	CompletedAt           *time.Time
	CancelledAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// OrderItem 订单明细。一条 Order 包含若干 OrderItem（每个 SKU 一行）。
// 商品名/价格/SKU 规格在下单时固化,后续 SKU 表变更不影响历史订单。
type OrderItem struct {
	ID                 int64     `gorm:"primaryKey"`
	OrderID            int64     `gorm:"not null;index:idx_order_item_order"`
	SkuID              int64     `gorm:"column:sku_id;not null"`
	ProductID          int64     `gorm:"not null"`
	ProductTitle       string    `gorm:"size:256"`
	ProductMainImage   string    `gorm:"size:512"`
	SkuAttrs           RawJSON   `gorm:"type:jsonb;not null;default:'{}'"`
	Qty                int       `gorm:"not null"`
	SnapshotPriceCents int64     `gorm:"not null"`
	CreatedAt          time.Time `gorm:"not null;default:now()"`
}

func (Order) TableName() string { return "order_main" }

func (OrderItem) TableName() string { return "order_item" }

func (FreightTemplate) TableName() string { return "freight_template" }

func (FreightTemplateRule) TableName() string { return "freight_template_rule" }

// FreightChargeType 运费计费方式。
type FreightChargeType string

const (
	FreightChargeByWeight FreightChargeType = "weight"
	FreightChargeByAmount FreightChargeType = "amount"
	FreightChargeFixed    FreightChargeType = "fixed"
)

// FreightTemplate 运费模板。
type FreightTemplate struct {
	ID        int64  `gorm:"primaryKey"`
	Name      string `gorm:"size:64;not null"`
	IsDefault bool   `gorm:"not null;default:false"`

	Rules []FreightTemplateRule `gorm:"foreignKey:TemplateID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// FreightRuleRegion 规则-地区多对多绑定（中间表）。
// 一条规则可绑定多个地区(省/市/区任意层级),每个绑定有 level 字段
// 表示该地区是哪个级别,匹配时按 区 > 市 > 省 > 默认 优先级排序。
type FreightRuleRegion struct {
	RuleID     int64  `gorm:"primaryKey;column:rule_id"`
	RegionCode string `gorm:"primaryKey;column:region_code;size:32"`
	Level      string `gorm:"primaryKey;column:level;size:8"` // 'province' | 'city' | 'district'
}

func (FreightRuleRegion) TableName() string { return "freight_rule_region" }

// FreightTemplateRule 运费规则（地区 × 运费组 × 计费方式）。
//
// 地区覆盖通过 FreightRuleRegion 多对多绑定,本表不再冗余存 province/city/district 字段。
// 同 level 内多条规则匹配按 Priority DESC 选最大;跨 level 时按 level 本身优先级(3=district > 2=city > 1=province > 0=default)。
type FreightTemplateRule struct {
	ID         int64 `gorm:"primaryKey"`
	TemplateID int64 `gorm:"not null;index"`

	// 可选：SKU 所属运费分组
	FreightGroupID *int64 `gorm:"index"`

	ChargeType FreightChargeType `gorm:"type:varchar(16);not null;default:'weight'"`

	// 满额免运，0 表示不启用
	FreeThresholdCents int64 `gorm:"not null;default:0"`

	// 按重量计费，单位：克
	FirstWeightG       *int   `gorm:"comment:首重克数"`
	FirstFeeCents      *int64 `gorm:"comment:首重费用分"`
	AdditionalWeightG  *int   `gorm:"comment:续重克数"`
	AdditionalFeeCents *int64 `gorm:"comment:续重费用分"`

	// 固定运费或其他计费方式
	FixedFeeCents *int64 `gorm:"comment:固定费用分"`

	// 数值越大优先级越高(同 level 内多条规则匹配时按 DESC 选最大)
	Priority int `gorm:"not null;default:0;index"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// 关联(规则 → 地区多对多)
	Regions []FreightRuleRegion `gorm:"foreignKey:RuleID"`

	Template FreightTemplate `gorm:"foreignKey:TemplateID"`
}
