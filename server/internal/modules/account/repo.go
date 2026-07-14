package account

import (
	"context"
	"shop/internal/pkg/snowflake"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepo interface {
	FindByOpenidMP(ctx context.Context, openid string) (*User, error)
	FindByOpenidH5(ctx context.Context, openid string) (*User, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	UpsertByOpenidMP(ctx context.Context, user *User) error
	UpsertByOpenidH5(ctx context.Context, user *User) error
	// UpsertByPhone 根据手机号插入或更新用户（设置密码场景）。
	UpsertByPhone(ctx context.Context, user *User) error
	// Create 创建新用户。
	Create(ctx context.Context, user *User) error
	// CountActiveByPhoneExclude 查询 status='active' 且手机号匹配、且 id != excludeUserID 的用户数量。
	CountActiveByPhoneExclude(ctx context.Context, phone string, excludeUserID int64) (int64, error)
	// CountByPhone 查询指定手机号的用户数量（不限制状态）。
	CountByPhone(ctx context.Context, phone string) (int64, error)
	Update(ctx context.Context, id int64, updates map[string]any) error
	// FindById 根据用户 ID 查询用户。
	FindById(ctx context.Context, id int64) (*User, error)
	// GetBalance 获取用户余额。
	GetBalance(ctx context.Context, id int64) (int64, error)
	// ListBalanceLogs 查询用户余额流水（分页）。
	ListBalanceLogs(ctx context.Context, userID int64, page, size int) ([]BalanceLog, int64, error)
	// ListUsers 查询后台用户列表（分页）。
	ListUsers(ctx context.Context, page, size int) ([]User, int64, error)
	// RechargeBalance 用户余额充值（仅在 admin 端使用）。
	RechargeBalance(ctx context.Context, userID, amountCents, operatorID int64, remark string) error
}

type userRepoImpl struct {
	db *gorm.DB
}

// NewUserRepo 构造 UserRepo 实现。
func NewUserRepo(db *gorm.DB) UserRepo {
	return &userRepoImpl{db: db}
}

func (r *userRepoImpl) FindByOpenidMP(ctx context.Context, openid string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).
		Where("openid_mp = ?", openid).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoImpl) FindByOpenidH5(ctx context.Context, openid string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).
		Where("openid_h5 = ?", openid).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoImpl) FindByPhone(ctx context.Context, phone string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).
		Where("phone = ?", phone).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoImpl) UpsertByOpenidMP(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "openid_mp"}},
			DoUpdates: clause.AssignmentColumns([]string{"unionid", "updated_at"}),
		}).
		Create(user).Error
}

func (r *userRepoImpl) UpsertByOpenidH5(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "openid_h5"}},
			DoUpdates: clause.AssignmentColumns([]string{"unionid", "source", "updated_at"}),
		}).
		Create(user).Error
}

func (r *userRepoImpl) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepoImpl) UpsertByPhone(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "phone"}},
			DoUpdates: clause.AssignmentColumns([]string{"password_hash", "updated_at"}),
		}).
		Create(user).Error
}

func (r *userRepoImpl) CountActiveByPhoneExclude(ctx context.Context, phone string, excludeUserID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("phone = ? AND status = ? AND id <> ?", phone, "active", excludeUserID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepoImpl) CountByPhone(ctx context.Context, phone string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&User{}).
		Where("phone = ?", phone).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepoImpl) Update(ctx context.Context, id int64, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *userRepoImpl) FindById(ctx context.Context, id int64) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).
		First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoImpl) GetBalance(ctx context.Context, id int64) (int64, error) {
	var balance int64
	if err := r.db.WithContext(ctx).
		Model(&User{}).
		Select("balance_cents").
		Where("id = ?", id).
		Take(&balance).Error; err != nil {
		return 0, err
	}
	return balance, nil
}

func (r *userRepoImpl) ListBalanceLogs(ctx context.Context, userID int64, page, size int) ([]BalanceLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var logs []BalanceLog
	var total int64

	query := r.db.WithContext(ctx).Model(&BalanceLog{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(size).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *userRepoImpl) ListUsers(ctx context.Context, page, size int) ([]User, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var users []User
	var total int64
	query := r.db.WithContext(ctx).Model(&User{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *userRepoImpl) RechargeBalance(ctx context.Context, userID, amountCents, operatorID int64, remark string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var u User
		if err := tx.Model(&User{}).Select("id ,balance_cents").First(&u, userID).Error; err != nil {
			return err
		}

		newBalance := amountCents + u.BalanceCents
		if err := tx.Model(&User{}).Where("id = ?", userID).Update("balance_cents", newBalance).Error; err != nil {
			return err
		}

		var opIDPtr *int64
		if operatorID > 0 {
			opIDPtr = &operatorID
		}
		bl := &BalanceLog{
			ID:                 snowflake.NextID(),
			UserID:             userID,
			ChangeCents:        amountCents,
			Type:               "recharge",
			BalanceBeforeCents: u.BalanceCents,
			BalanceAfterCents:  newBalance,
			OperatorID:         opIDPtr,
			Remark:             &remark,
		}
		return tx.Create(bl).Error
	})
}

// -------- AdminRepo --------
type AdminRepo interface {
	// FindByUserName 根据用户名查询管理员。
	FindByUserName(ctx context.Context, username string) (*Admin, error)
	// FindByID 根据 ID 查询管理员。
	FindByID(ctx context.Context, id int64) (*Admin, error)
	// Update 根据 ID 更新管理员指定字段。
	Update(ctx context.Context, id int64, updates map[string]any) error
	// ListAdmins 查询管理员列表（分页）。
	ListAdmins(ctx context.Context, page, size int) ([]Admin, int64, error)
	// CountByPhone 查询指定手机号的管理员数量。
	CountByPhone(ctx context.Context, phone string) (int64, error)
	// Create 创建管理员。
	Create(ctx context.Context, admin *Admin) error
}

// ------- AdminRepoImpl -------

type adminRepoImpl struct{ db *gorm.DB }

// NewAdminRepo 构造 AdminRepo 实现。
func NewAdminRepo(db *gorm.DB) AdminRepo {
	return &adminRepoImpl{db: db}
}

func (r *adminRepoImpl) FindByUserName(ctx context.Context, username string) (*Admin, error) {
	var admin Admin
	if err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepoImpl) Update(ctx context.Context, id int64, updates map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&Admin{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *adminRepoImpl) FindByID(ctx context.Context, id int64) (*Admin, error) {
	var admin Admin
	if err := r.db.WithContext(ctx).
		First(&admin, id).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *adminRepoImpl) ListAdmins(ctx context.Context, page, size int) ([]Admin, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	var admins []Admin
	var total int64

	query := r.db.WithContext(ctx).Model(&Admin{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(size).
		Find(&admins).Error; err != nil {
		return nil, 0, err
	}

	return admins, total, nil
}

func (r *adminRepoImpl) CountByPhone(ctx context.Context, phone string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Admin{}).
		Where("phone = ?", phone).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *adminRepoImpl) Create(ctx context.Context, admin *Admin) error {
	return r.db.WithContext(ctx).Create(admin).Error
}

// -------- RoleRepo --------
type RoleRepo interface {
	// GetAdminRoleCodes 查询管理员角色编码。
	GetAdminRoleCodes(ctx context.Context, adminID int64) ([]string, error)
	// GetAdminPermCodes 查询管理员拥有的所有权限编码（通过角色继承）。
	GetAdminPermCodes(ctx context.Context, adminID int64) ([]string, error)
	// FindRolesByIDs 根据 ID 列表查询角色。
	FindRolesByIDs(ctx context.Context, ids []int64) ([]Role, error)
	// BatchBindRoles 批量绑定管理员角色。
	BatchBindRoles(ctx context.Context, adminID int64, roleIDs []int64) error
	ListRoles(ctx context.Context) ([]Role, error)
	FindRoleByID(ctx context.Context, id int64) (*Role, error)
	CreateRole(ctx context.Context, role *Role, permissionCodes []string) error
	UpdateRole(ctx context.Context, id int64, name *string, permissionCodes *[]string) error
	DeleteRole(ctx context.Context, id int64) error
	ListPermissions(ctx context.Context) ([]Permission, error)
}

type roleRepoImpl struct{ db *gorm.DB }

func NewRoleRepo(db *gorm.DB) RoleRepo {
	return &roleRepoImpl{db: db}
}

func (r *roleRepoImpl) GetAdminRoleCodes(ctx context.Context, adminID int64) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).
		Model(&AdminRole{}).
		Where("admin_id = ?", adminID).
		Pluck("role_code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *roleRepoImpl) GetAdminPermCodes(ctx context.Context, adminID int64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT rp.permission_code
		FROM admin_role ar
		JOIN role r ON ar.role_code = r.code
		JOIN role_permission rp ON rp.role_id = r.id
		WHERE ar.admin_id = ?
	`, adminID).Scan(&codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *roleRepoImpl) FindRolesByIDs(ctx context.Context, ids []int64) ([]Role, error) {
	var roles []Role
	if err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepoImpl) BatchBindRoles(ctx context.Context, adminID int64, roleIDs []int64) error {
	var adminRoles []AdminRole
	for _, rid := range roleIDs {
		// 需要先查到 role 的 code
		var role Role
		if err := r.db.WithContext(ctx).First(&role, rid).Error; err != nil {
			return err
		}
		adminRoles = append(adminRoles, AdminRole{
			AdminID:  adminID,
			RoleCode: role.Code,
		})
	}
	return r.db.WithContext(ctx).Create(&adminRoles).Error
}

func (r *roleRepoImpl) ListRoles(ctx context.Context) ([]Role, error) {
	var roles []Role
	if err := r.db.WithContext(ctx).Preload("Permissions").Order("created_at ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepoImpl) FindRoleByID(ctx context.Context, id int64) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Preload("Permissions").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepoImpl) CreateRole(ctx context.Context, role *Role, permissionCodes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		return replaceRolePermissions(tx, role.ID, permissionCodes)
	})
}

func (r *roleRepoImpl) UpdateRole(ctx context.Context, id int64, name *string, permissionCodes *[]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if name != nil {
			if err := tx.Model(&Role{}).Where("id = ?", id).Update("name", *name).Error; err != nil {
				return err
			}
		}
		if permissionCodes != nil {
			return replaceRolePermissions(tx, id, *permissionCodes)
		}
		return nil
	})
}

func (r *roleRepoImpl) DeleteRole(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Role{}, id).Error
	})
}

func (r *roleRepoImpl) ListPermissions(ctx context.Context) ([]Permission, error) {
	var permissions []Permission
	if err := r.db.WithContext(ctx).Order("module ASC, action ASC, code ASC").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func replaceRolePermissions(tx *gorm.DB, roleID int64, permissionCodes []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
		return err
	}
	if len(permissionCodes) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&Permission{}).Where("code IN ?", permissionCodes).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(permissionCodes)) {
		return gorm.ErrRecordNotFound
	}
	links := make([]RolePermission, 0, len(permissionCodes))
	for _, code := range permissionCodes {
		links = append(links, RolePermission{RoleID: roleID, PermissionCode: code})
	}
	return tx.Create(&links).Error
}
