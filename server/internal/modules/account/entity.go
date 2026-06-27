// Package account 实现账号和权限相关的领域逻辑。
package account

import "time"

// User C 端用户表。
type User struct {
	ID              int64      `gorm:"primaryKey"`                       // 主键
	OpenidMP        *string    `gorm:"column:openid_mp;uniqueIndex;size:64"`  // 小程序id
	OpenidH5        *string    `gorm:"column:openid_h5;uniqueIndex;size:64"`  // 公众号id
	Unionid         *string    `gorm:"column:unionid;uniqueIndex;size:64"`    // 微信开放平台统一ID
	Phone           *string    `gorm:"column:phone;uniqueIndex;size:20"`
	PhoneCountry    string     `gorm:"column:phone_country;default:86;size:5"`
	PasswordHash    *string    `gorm:"column:password_hash;size:255"`
	Source          string     `gorm:"column:source;default:mp;size:10"` // 用户登录平台
	NickName        *string    `gorm:"column:nickname;size:50"`          // 昵称
	Avatar          *string    `gorm:"column:avatar;size:256"`
	Gender          int        `gorm:"column:gender;default:0"`
	Birthday        *time.Time `gorm:"column:birthday"`
	Status          string     `gorm:"column:status;default:active;size:20"`
	DeactivateAt    *time.Time `gorm:"column:deactivate_at"`      // 软注销
	InvitedByUserID *int64     `gorm:"column:invited_by_user_id"` // 邀请人
	DistributorID   *int64     `gorm:"column:distributor_id"`     // 分销员id
	Points          int        `gorm:"column:points;default:0"`
	BalanceCents    int64      `gorm:"column:balance_cents;not null;default:0"` // 余额，以分为单位，避免小数计算
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 对应 PostgreSQL 的 "user" 表（关键字需加引号）。
func (User) TableName() string { return `"user"` }
