package region

import "time"

// Region 行政区划实体。
type Region struct {
	ID        int64     `gorm:"primaryKey"`
	Code      string    `gorm:"size:32;not null;uniqueIndex:uk_region_code"`
	ParentID  int64     `gorm:"not null;default:0;index:idx_region_parent_id"`
	Name      string    `gorm:"size:64;not null"`
	Level     int       `gorm:"not null;default:1"`
	Sort      int       `gorm:"not null;default:0"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
	// HasChildren 仅在 ListByParentID 查询结果中填充（gorm:"-" 不落库）。
	HasChildren bool `gorm:"-"`
}

func (Region) TableName() string { return "region" }