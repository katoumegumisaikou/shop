package account

import "gorm.io/gorm"

type UserRepo struct {
}

type UserRepoImpl struct {
	db gorm.DB
}
