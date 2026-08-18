package order

import (
	"context"

	"go.uber.org/zap"

	"shop/internal/modules/product"
)

// OrderRepo 订单数据访问接口。
//
// 占位定义，方法签名待 entity.go / repo.go 落地后再补全。
// 预期至少包含：
//   - InsertOrder(ctx, *Order, []*OrderItem) error
//   - FindSKUForOrder(ctx, skuID) (*OrderSKUInfo, error)
//   - GetByID(ctx, id) (*Order, error)
//   - ListByUserID(ctx, userID, filter) ([]*Order, int, error)
//   - UpdateStatus(ctx, id, status) error
type OrderRepo interface{}

// Service 订单服务。
type Service struct {
	repo        OrderRepo
	productRepo product.ProductRepo
	logger      *zap.Logger
}

// NewService 构造 Service。
func NewService(repo OrderRepo, productRepo product.ProductRepo, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		repo:        repo,
		productRepo: productRepo,
		logger:      logger,
	}
}

// CreateOrder 下单。
//
// 占位实现，TODO：等 entity.go / repo.go / freight / coupon 模块接入后再实现：
//  1. 生成 order_id (snowflake) + order_no (yyyyMMddHHmmss + 6 位随机)
//  2. 逐行查 SKU + 商品状态、计算总金额
//  3. 计算运费 / 优惠
//  4. 事务内插入 order_main + order_item
//  5. 清理 from_cart_item_ids 对应的购物车条目
//  6. 返回 {order_id, order_no}
func (s *Service) CreateOrder(ctx context.Context, user_id int64, req *CreateOrderReq) (*CreateOrderResp, error) {

	return &CreateOrderResp{}, nil
}
