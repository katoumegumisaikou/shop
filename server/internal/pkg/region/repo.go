package region

import (
	"context"

	"gorm.io/gorm"
)

// Repo 行政区划数据访问接口。
type Repo interface {
	// GetByCode 按行政编码查询。
	GetByCode(ctx context.Context, code string) (*Region, error)
	// ListByParentCode 列出指定 parent_code 的直接子级。
	//   - parentCode == "" 时返回顶级（省），用 WHERE parent_id = 0 命中
	//   - parentCode 非空时 JOIN 父级表，WHERE p.code = ? 命中
	// 单条 SQL 同时拿 region 字段 + HasChildren，避免先查 id 再查 children 的 2 次往返。
	ListByParentCode(ctx context.Context, parentCode string) ([]Region, error)
}

type repoImpl struct{ db *gorm.DB }

// NewRepo 构造 Repo。
func NewRepo(db *gorm.DB) Repo {
	return &repoImpl{db: db}
}

// GetByCode 按 code 查单个 region。
func (r *repoImpl) GetByCode(ctx context.Context, code string) (*Region, error) {
	var region Region
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&region).Error
	if err != nil {
		return nil, err // gorm.ErrRecordNotFound 自然冒泡
	}
	return &region, nil
}

// ListByParentCode 单条 SQL 同时拿到 region 字段 + HasChildren。
//   - 顶级: parent_id = 0（直接命中）
//   - 子级: JOIN region p ON p.id = r.parent_id WHERE p.code = ?
func (r *repoImpl) ListByParentCode(ctx context.Context, parentCode string) ([]Region, error) {
	var regions []Region
	q := r.db.WithContext(ctx).
		Table("region AS r").
		Select(`r.*,
			EXISTS(SELECT 1 FROM region c WHERE c.parent_id = r.id) AS has_children`)

	if parentCode == "" {
		q = q.Where("r.parent_id = 0")
	} else {
		q = q.Joins("JOIN region p ON p.id = r.parent_id").Where("p.code = ?", parentCode)
	}

	err := q.Order("r.sort ASC, r.id ASC").Find(&regions).Error
	return regions, err
}