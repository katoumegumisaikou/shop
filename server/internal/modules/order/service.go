package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"shop/internal/modules/address"
	"shop/internal/modules/product"
	"shop/internal/pkg/errs"
	"shop/internal/pkg/region"
)

// AddressLookup 收货地址最小查询接口。
// address.AddressRepo 自动满足此接口(Go 结构化类型),无需在 address 模块改任何代码。
// 只声明"下单需要的方法",符合接口隔离原则。
type AddressLookup interface {
	GetByID(ctx context.Context, id int64) (*address.Address, error)
}

// OrderSKUInfo 给下单用的轻量 SKU + Product 视图。
// SQL JOIN sku 与 product 一次拉全,避免应用层二次查询。
// 字段集与 cart.CartSKUInfo 保持一致(都是下单/购物车场景),后续可考虑抽到 common 包复用。
type OrderSKUInfo struct {
	SkuID            int64
	ProductID        int64
	ProductTitle     string
	ProductMainImage string
	ProductStatus    string
	ProductDeleted   bool
	SkuAttrs         []byte
	SkuPriceCents    int64
	SkuStatus        string
	SkuStock         int
	SkuLockedStock   int
}

// OrderRepo 订单数据访问接口。
type OrderRepo interface {
	// InsertOrder 事务内插入订单主表 + 明细表。
	InsertOrder(ctx context.Context, o *Order, items []*OrderItem) error
	// FindSKUsByIDs 批量按 SKU ID 查 SKU + Product 信息,返回以 skuID 为键的 map。
	// 不存在的 skuID 不在 map 里(调用方需自行判 missing)。
	FindSKUsByIDs(ctx context.Context, skuIDs []int64) (map[int64]*OrderSKUInfo, error)
	// GetByID 按主键查订单。
	GetByID(ctx context.Context, id int64) (*Order, error)
	// ListByUserID 分页查用户的订单。
	ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*Order, int, error)
	// UpdateStatus 更新订单状态。
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// Service 订单服务。
type Service struct {
	repo        OrderRepo
	productRepo product.ProductRepo
	addrLookup  AddressLookup
	regionRepo  region.Repo
	rdb         *redis.Client
	logger      *zap.Logger
}

// NewService 构造 Service。
func NewService(repo OrderRepo, productRepo product.ProductRepo, addrLookup AddressLookup, regionRepo region.Repo, rdb *redis.Client, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		repo:        repo,
		productRepo: productRepo,
		addrLookup:  addrLookup,
		regionRepo:  regionRepo,
		rdb:         rdb,
		logger:      logger,
	}
}

// getAddressWithOwnership 查地址并校验归属。
// 校验:地址必须存在 + 必须属于当前用户,否则 ErrNotFound(不暴露区分)。
// 返回的地址直接用作 Order.AddressSnapshot 的来源(下单时固化,不随地址变更)。
func (s *Service) getAddressWithOwnership(ctx context.Context, userID, addrID int64) (*address.Address, error) {
	addr, err := s.addrLookup.GetByID(ctx, addrID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		s.logger.Error("get address failed", zap.Int64("address_id", addrID), zap.Error(err))
		return nil, errs.ErrInternal
	}
	if addr.UserID != userID {
		return nil, errs.ErrForbidden
	}
	return addr, nil
}

// buildAddressSnapshot 把 address.Address + region 链路组装成 AddressSnapshot。
//
// 关键:AddressSnapshot 在下单时**固化**(snapshot),后续用户修改原地址不影响历史订单。
// 三段(省/市/区)通过 regionRepo.GetRegionChain 一次 SQL 拿到,避免 3 次循环查询。
func (s *Service) buildAddressSnapshot(ctx context.Context, addr *address.Address) (AddressSnapshot, error) {
	chain, err := s.regionRepo.GetRegionChain(ctx, addr.RegionCode)
	if err != nil {
		s.logger.Error("get region chain failed", zap.String("region_code", addr.RegionCode), zap.Error(err))
		return AddressSnapshot{}, errs.ErrInternal
	}
	if chain.DistrictCode == "" {
		// region_code 查不到 → 地址表里的 region_code 是脏数据
		return AddressSnapshot{}, errs.ErrParam.WithMsg(fmt.Sprintf("地址区域 %s 无效", addr.RegionCode))
	}

	return AddressSnapshot{
		Name:         addr.ReceiverName,
		Phone:        addr.Phone,
		Province:     chain.ProvinceName,
		ProvinceCode: chain.ProvinceCode,
		City:         chain.CityName,
		CityCode:     chain.CityCode,
		District:     chain.DistrictName,
		DistrictCode: addr.RegionCode,
		// Street/StreetCode 留空(address 模块当前无街道字段)
		Detail: addr.Detail,
	}, nil
}

// checkItemsAvailability 校验所有下单商品的可用性。
//
// 校验项（顺序敏感，先查"是否存在"再查"是否上架"再查"库存"）：
//  1. SKU 必须能查到(在 skuMap 里)
//  2. 商品未软删除(ProductDeleted == false)
//  3. 商品已上架(ProductStatus == "onsale")
//  4. SKU 状态激活(SkuStatus == "active")
//  5. 库存充足(SkuStock - SkuLockedStock >= 购买数量)
//
// 任意一项不满足,返回带具体 skuID 和原因的 AppError;全部通过返回 nil。
// 同一 sku 多行(分多次加购)自动累加 qty 再校验。
func (s *Service) checkItemsAvailability(req *CreateOrderReq, skuMap map[int64]*OrderSKUInfo) error {
	// 聚合 qty：同一 sku 多行合并计算
	qtyByID := make(map[int64]int, len(req.Items))
	for _, it := range req.Items {
		qtyByID[int64(it.SkuID)] += it.Qty
	}

	for skuID, qty := range qtyByID {
		info, ok := skuMap[skuID]
		if !ok {
			return errs.ErrNotFound.WithMsg(fmt.Sprintf("SKU %d 不存在", skuID))
		}
		if info.ProductDeleted {
			return errs.ErrParam.WithMsg(fmt.Sprintf("SKU %d 对应商品已删除", skuID))
		}
		if info.ProductStatus != "onsale" {
			return errs.ErrParam.WithMsg(fmt.Sprintf("SKU %d 对应商品未上架", skuID))
		}
		if info.SkuStatus != "active" {
			return errs.ErrParam.WithMsg(fmt.Sprintf("SKU %d 已下架", skuID))
		}
		available := info.SkuStock - info.SkuLockedStock
		if available < qty {
			if available <= 0 {
				return errs.ErrParam.WithMsg(fmt.Sprintf("SKU %d 已无库存", skuID))
			}
			return errs.ErrParam.WithMsg(fmt.Sprintf("SKU %d 库存不足，最多可购买 %d 件", skuID, available))
		}
	}
	return nil
}

// CreateOrder 下单。
//
// 占位实现，TODO：等 freight / coupon 模块接入后再实现运费/优惠计算。
// 当前 demo 阶段只演示 SKU 拉取 + 校验 + 写库主流程。
func (s *Service) CreateOrder(ctx context.Context, userID int64, req *CreateOrderReq) (*Order, error) {
	_ = userID

	// 1. 收集所有 skuID（去重）
	skuIDSet := make(map[int64]struct{}, len(req.Items))
	for _, it := range req.Items {
		skuIDSet[int64(it.SkuID)] = struct{}{}
	}
	// note:去重
	skuIDs := make([]int64, 0, len(skuIDSet))
	for id := range skuIDSet {
		skuIDs = append(skuIDs, id)
	}

	// 2. 一次 SQL 拉所有 SKU + Product
	skuMap, err := s.repo.FindSKUsByIDs(ctx, skuIDs)
	if err != nil {
		s.logger.Error("find skus failed", zap.Error(err))
		return nil, err
	}

	// 3. 校验商品可用性（状态 / 删除 / 库存）
	if err := s.checkItemsAvailability(req, skuMap); err != nil {
		return nil, err
	}

	// 4. 计算运费
	//  查地址并校验归属(归属校验防越权下单到别人地址)
	addr, err := s.getAddressWithOwnership(ctx, userID, int64(req.AddressID))
	if err != nil {
		return nil, err
	}

	//  根据地址的区域查询构建完整的地址快照
	snap, err := s.buildAddressSnapshot(ctx, addr)
	if err != nil {
		return nil, err
	}
	//  根据地址快照查找运费模板
	s.regionRepo

	_ = snap // TODO: 写入 Order.AddressSnapshot

	// TODO: 计算总金额 / 算运费 / 写 order_main + order_item / 清理购物车
	return &Order{}, nil
}

func calcFreight(adder *AddressSnapshot)
