package product

import "github.com/redis/go-redis/v9"

type Service struct {
	rdb         *redis.Client
	productRepo CategoryRepo
}

func NewService(rdb *redis.Client, productRepo Category) *Service {
	return &Service{rdb: rdb, productRepo: productRepo}
}

func (s *Service) ListCategories() {

}
