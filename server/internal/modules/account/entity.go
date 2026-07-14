// Package account 实现账号和权限相关的领域逻辑。
package account

import "time"

// User C 端用户表。
type User struct {
	ID              int64      `gorm:"primaryKey"`                           // 主键
	OpenidMP        *string    `gorm:"column:openid_mp;uniqueIndex;size:64"` // 小程序id
	OpenidH5        *string    `gorm:"column:openid_h5;uniqueIndex;size:64"` // 公众号id
	Unionid         *string    `gorm:"column:unionid;uniqueIndex;size:64"`   // 微信开放平台统一ID
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
	DeletedAt       *time.Time `gorm:"index"`
}

// TableName 对应 PostgreSQL 的 "user" 表（关键字需加引号）。
func (User) TableName() string { return `"user"` }

// BalanceLog 余额流水。
type BalanceLog struct {
	ID                 int64     `gorm:"primaryKey"`
	UserID             int64     `gorm:"column:user_id;not null"`
	ChangeCents        int64     `gorm:"column:change_cents;not null"`         // 变动金额，单位：分
	Type               string    `gorm:"column:type;size:16;not null"`         // 变动类型
	RefType            *string   `gorm:"column:ref_type;size:16"`              // 关联业务类型
	RefID              *int64    `gorm:"column:ref_id"`                        // 关联业务 ID
	BalanceBeforeCents int64     `gorm:"column:balance_before_cents;not null"` // 变动前余额
	BalanceAfterCents  int64     `gorm:"column:balance_after_cents;not null"`  // 变动后余额
	OperatorID         *int64    `gorm:"column:operator_id"`                   // 操作人
	Remark             *string   `gorm:"column:remark;size:200"`               // 备注
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (BalanceLog) TableName() string { return "balance_log" }

type Admin struct {
	ID             int64      `gorm:"primaryKey"`
	Username       string     `gorm:"column:username;uniqueIndex;not null"`
	PasswordHash   string     `gorm:"column:password_hash;not null"`
	RealName       *string    `gorm:"column:real_name"`
	Phone          *string    `gorm:"column:phone"`
	Status         string     `gorm:"column:status;default:active"`
	FailedAttempts int        `gorm:"column:failed_attempts;default:0"`
	LockedUntil    *time.Time `gorm:"column:locked_until"`
	LastLoginAt    *time.Time `gorm:"column:last_login_at"`
	LastLoginIP    *string    `gorm:"column:last_login_ip"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt      *time.Time `gorm:"column:deleted_at;index"`
}

func (Admin) TableName() string { return "admin" }

// Role 角色表。
type Role struct {
	ID        int64     `gorm:"primaryKey"`
	Code      string    `gorm:"column:code;uniqueIndex;not null"`
	Name      string    `gorm:"column:name;not null"`
	IsSystem  bool      `gorm:"column:is_system;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`

	Permissions []Permission `gorm:"many2many:role_permission;joinForeignKey:role_id;joinReferences:permission_code"`
}

func (Role) TableName() string { return "role" }

// Permission 权限点表。
type Permission struct {
	Code   string `gorm:"primaryKey"`
	Module string `gorm:"column:module;not null"`
	Action string `gorm:"column:action;not null"`
	Name   string `gorm:"column:name;not null"`
}

func (Permission) TableName() string { return "permission" }

// AdminRole 管理员角色关联。
type AdminRole struct {
	AdminID  int64  `gorm:"column:admin_id;primaryKey"`
	RoleCode string `gorm:"column:role_code;primaryKey;size:32"`
}

func (AdminRole) TableName() string { return "admin_role" }

// AdminPerm 管理员权限点（独立于角色的额外权限）。
type AdminPerm struct {
	AdminID  int64  `gorm:"column:admin_id;primaryKey"`
	PermCode string `gorm:"column:perm_code;primaryKey;size:64"`
}

func (AdminPerm) TableName() string { return "admin_perm" }

// RolePermission 角色-权限关联表。
type RolePermission struct {
	RoleID         int64  `gorm:"primaryKey;column:role_id"`
	PermissionCode string `gorm:"primaryKey;column:permission_code"`
}

func (RolePermission) TableName() string { return "role_permission" }
