package account

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepo interface {
	FindByOpenidMP(ctx context.Context, openid string) (*User, error)
	FindByOpenidH5(ctx context.Context, openid string) (*User, error)
	UpsertByOpenidMP(ctx context.Context, user *User) error
	UpsertByOpenidH5(ctx context.Context, user *User) error
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
