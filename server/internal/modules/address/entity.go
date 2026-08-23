package address

import "time"

// Address 收货地址实体。
type Address struct {
	ID           int64     `gorm:"primaryKey"`
	UserID       int64     `gorm:"not null;index:idx_addr_user"`
	ReceiverName string    `gorm:"size:64;not null"`
	Phone        string    `gorm:"size:20;not null"`
	RegionCode   string    `gorm:"size:32;not null"` // 存的是区/县级的行政区 region.code，如 "110100"
	Detail       string    `gorm:"size:256;not null"`
	IsDefault    bool      `gorm:"not null;default:false"`
	CreatedAt    time.Time `gorm:"not null;default:now()"`
	UpdatedAt    time.Time `gorm:"not null;default:now()"`
}

func (Address) TableName() string { return "address" }
