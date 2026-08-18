package address

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// AddressRepo 收货地址数据访问接口。
type AddressRepo interface {
	// InsertAddress 插入新地址。若 IsDefault=true，事务内先清同用户其他默认。
	InsertAddress(ctx context.Context, a *Address) error
	// GetByID 按主键查询。
	GetByID(ctx context.Context, id int64) (*Address, error)
	// ListByUserID 列出指定用户全部地址。
	ListByUserID(ctx context.Context, userID int64) ([]*Address, error)
	// UpdateAddress 更新指定地址。若 IsDefault=true，事务内先清同用户其他默认。
	UpdateAddress(ctx context.Context, a *Address) error
	// DeleteByUserID 仅当记录归属 userID 时删除（防越权），返回影响行数。
	DeleteByUserID(ctx context.Context, userID, id int64) (int64, error)
	// SetDefault 事务内：先 ClearDefault，再把指定 id 设为默认。返回影响行数。
	SetDefault(ctx context.Context, userID, id int64) (int64, error)
}

type addressRepoImpl struct{ db *gorm.DB }

// NewAddressRepo 构造 AddressRepo。
func NewAddressRepo(db *gorm.DB) AddressRepo {
	return &addressRepoImpl{db: db}
}

// InsertAddress 插入新地址。若 IsDefault=true，事务内先清同用户其他默认。
func (r *addressRepoImpl) InsertAddress(ctx context.Context, a *Address) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if a.IsDefault {
			if err := tx.Model(&Address{}).
				Where("user_id = ?", a.UserID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(a).Error
	})
}

func (r *addressRepoImpl) GetByID(ctx context.Context, id int64) (*Address, error) {
	var a Address
	if err := r.db.WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err // gorm.ErrRecordNotFound 自然冒泡
	}
	return &a, nil
}

func (r *addressRepoImpl) ListByUserID(ctx context.Context, userID int64) ([]*Address, error) {
	var list []*Address
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_default DESC, id DESC").
		Find(&list).Error
	return list, err
}

// UpdateAddress 更新指定地址。若 IsDefault=true，事务内先清同用户其他默认。
func (r *addressRepoImpl) UpdateAddress(ctx context.Context, a *Address) error {
	a.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if a.IsDefault {
			// 排除自身：避免与"原行已为 true"过渡时的自我清除
			if err := tx.Model(&Address{}).
				Where("user_id = ? AND id <> ?", a.UserID, a.ID).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(&Address{}).
			Where("id = ?", a.ID).
			Updates(map[string]any{
				"receiver_name": a.ReceiverName,
				"phone":         a.Phone,
				"region_code":   a.RegionCode,
				"detail":        a.Detail,
				"is_default":    a.IsDefault,
				"updated_at":    a.UpdatedAt,
			}).Error
	})
}

func (r *addressRepoImpl) DeleteByUserID(ctx context.Context, userID, id int64) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&Address{})
	return res.RowsAffected, res.Error
}

// SetDefault 事务内把 (userID, id) 设为默认：先清其他默认,再置当前默认。
// 返回的 int64 是 Update 的影响行数（用于 service 判断 id 是否归属 userID）。
func (r *addressRepoImpl) SetDefault(ctx context.Context, userID, id int64) (int64, error) {
	var rows int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Address{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		res := tx.Model(&Address{}).
			Where("id = ? AND user_id = ?", id, userID).
			Update("is_default", true)
		if res.Error != nil {
			return res.Error
		}
		rows = res.RowsAffected
		return nil
	})
	return rows, err
}