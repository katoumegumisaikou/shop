package product

import "gorm.io/gorm"

// ---- CategoryRepo ----

// 分类数据访问接口。
type CategoryRepo interface {
}

type categoryRepoImpl struct{ db *gorm.DB }

// NewCategoryRepo 构造 CategoryRepo。
func NewCategoryRepo(db *gorm.DB) CategoryRepo {
	return &categoryRepoImpl{db: db}
}
