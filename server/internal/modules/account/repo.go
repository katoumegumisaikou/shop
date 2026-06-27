package account

import (
	"context"

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
