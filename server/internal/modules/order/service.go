package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	SkuID             int64
	ProductID         int64
	ProductTitle      string
	ProductMainImage  string
	ProductStatus     string
	ProductDeleted    bool
	SkuAttrs          []byte
	SkuPriceCents     int64
	SkuStatus         string
	SkuStock          int
	SkuLockedStock    int
	WeightG           int
	FreightTemplateID *int64
}

// Service 订单服务。
type Service struct {
	repo          OrderRepo
	productRepo   product.ProductRepo
	addrLookup    AddressLookup
	regionRepo    region.Repo
	thresholdRepo FreightThresholdRepo
	rdb           *redis.Client
	logger        *zap.Logger
}

// NewService 构造 Service。
func NewService(repo OrderRepo, productRepo product.ProductRepo, addrLookup AddressLookup, regionRepo region.Repo, thresholdRepo FreightThresholdRepo, rdb *redis.Client, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		repo:          repo,
		productRepo:   productRepo,
		addrLookup:    addrLookup,
		regionRepo:    regionRepo,
		thresholdRepo: thresholdRepo,
		rdb:           rdb,
		logger:        logger,
	}
}

// getOrderFreeThreshold 查整单免运阈值(分币种)。
//
// 流程:
//  1. 判偏远(isRemoteArea 函数,无需 DB)
//  2. 按是否偏远查 freight_order_threshold 表
//  3. 返回 threshold_cents(分);0 表示未配置/出错 → 不启用整单免运
//
// 失败处理:DB 错误只 log,不返回 error — 整单免运是"优化项",失败时降级到"按 product 算"。
func (s *Service) getOrderFreeThreshold(ctx context.Context, snap AddressSnapshot) int64 {
	if s.thresholdRepo == nil {
		return 0
	}
	remote := isRemoteArea(snap.ProvinceCode, snap.CityCode)
	t, err := s.thresholdRepo.GetByRemote(ctx, remote)
	if err != nil {
		s.logger.Warn("get order free threshold failed",
			zap.Bool("is_remote", remote), zap.Error(err))
		return 0
	}
	if t == nil {
		// 未配置 → 不启用整单免运
		return 0
	}
	return t.ThresholdCents
}

// remoteProvinces 整省偏远的白名单(省级 code)。
// 整省偏远:该省所有地址都按偏远处理(运贵/时间长)。
var remoteProvinces = map[string]struct{}{
	"26": {}, // 西藏(整省偏远)
	"31": {}, // 新疆(整省偏远)
	"29": {}, // 青海(整省偏远)
	"11": {}, // 内蒙古(整省偏远)
	"28": {}, // 甘肃(整省偏远)
	"30": {}, // 宁夏(整省偏远)
}

// remoteCities 单城市偏远的白名单(市级 code)。
// 这些城市虽然所属省份不偏远,但因地理位置/物流条件单独算偏远(运费高)。
var remoteCities = map[string]struct{}{
	"2084": {}, // 四川-甘孜州
	"2103": {}, // 四川-凉山州
	"2070": {}, // 四川-阿坝州
}

// isRemoteArea 判断一个地址是否属偏远地区。
// 同时检查省级 + 市级 code,任一命中即视为偏远:
//
//	isRemoteArea("26", "")     // 西藏某区   → true (整省偏远)
//	isRemoteArea("22", "2103")  // 四川凉山州 → true (单市偏远)
//	isRemoteArea("19", "")     // 湖南某市   → false
//
// 优先级:任一命中即 true,无需复杂判断。
func isRemoteArea(provinceCode, cityCode string) bool {
	if provinceCode != "" {
		if _, ok := remoteProvinces[provinceCode]; ok {
			return true
		}
	}
	if cityCode != "" {
		if _, ok := remoteCities[cityCode]; ok {
			return true
		}
	}
	return false
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

// calcFreight 按地址快照匹配规则，并按运费模板合并计算运费。
//
// 流程:
//  1. 整单免运检查:查 freight_order_threshold 全局阈值,orderTotal ≥ 阈值 → return 0
//  2. 否则按 product 维度算:每个 template 一个 bucket,内部按"区 > 市 > 省"匹配规则
func (s *Service) calcFreight(ctx context.Context, skuMap map[int64]*OrderSKUInfo, qtyByID map[int64]int, templates map[int64]*FreightTemplate, snap AddressSnapshot, orderTotalCents int64) (int64, error) {
	// 1. 整单免运检查(全局阈值,按地址偏远类别查)
	threshold := s.getOrderFreeThreshold(ctx, snap)
	if threshold > 0 && orderTotalCents >= threshold {
		return 0, nil
	}

	// 2. 按 product 维度算
	type bucket struct {
		template *FreightTemplate
		goods    int64
		weight   int64
	}
	buckets := make(map[int64]*bucket)
	for skuID, qty := range qtyByID {
		info := skuMap[skuID]
		template := templates[freightTemplateID(info)]
		if info == nil || template == nil || qty <= 0 {
			continue
		}
		id := template.ID
		b := buckets[id]
		if b == nil {
			b = &bucket{template: template}
			buckets[id] = b
		}
		b.goods += info.SkuPriceCents * int64(qty)
		b.weight += int64(info.WeightG) * int64(qty)
	}

	var total int64
	for _, b := range buckets {
		rule, err := s.loadMatchedRule(ctx, b.template.ID, snap)
		if err != nil {
			return 0, err
		}
		if rule == nil || (rule.FreeThresholdCents > 0 && b.goods >= rule.FreeThresholdCents) {
			continue
		}
		switch rule.ChargeType {
		case FreightChargeFixed:
			// 固定运费:不管满不满,直接收 FixedFeeCents
			if rule.FixedFeeCents != nil && *rule.FixedFeeCents > 0 {
				total += *rule.FixedFeeCents
			}
		case FreightChargeByAmount:
			// 按金额计费:该组商品金额满 X 免,否则收 FixedFeeCents
			if rule.FreeThresholdCents > 0 && b.goods >= rule.FreeThresholdCents {
				continue
			}
			if rule.FixedFeeCents != nil && *rule.FixedFeeCents > 0 {
				total += *rule.FixedFeeCents
			}
		case FreightChargeByWeight:
			if rule.FirstWeightG == nil || rule.FirstFeeCents == nil || *rule.FirstWeightG <= 0 {
				continue
			}
			fee := *rule.FirstFeeCents
			if b.weight > int64(*rule.FirstWeightG) && rule.AdditionalWeightG != nil && rule.AdditionalFeeCents != nil && *rule.AdditionalWeightG > 0 {
				extra := b.weight - int64(*rule.FirstWeightG)
				fee += (extra + int64(*rule.AdditionalWeightG) - 1) / int64(*rule.AdditionalWeightG) * *rule.AdditionalFeeCents
			}
			if fee > 0 {
				total += fee
			}
		}
	}
	return total, nil
}

func freightTemplateID(info *OrderSKUInfo) int64 {
	if info == nil || info.FreightTemplateID == nil {
		return 0
	}
	return *info.FreightTemplateID
}

// loadMatchedRule 加载"模板在指定地址下应使用的规则"(带 Redis 缓存)。
//
// 缓存 key 沿用地址粒度设计：普通地区使用区级 code，偏远地区额外使用街道 code。
// 模板运营变更后需要删除对应 key，默认 TTL 为 1 小时。
//
// 返回 nil = 该地址没有匹配规则(不缓存,后续配置变了可能就有)。
func (s *Service) loadMatchedRule(ctx context.Context, templateID int64, snap AddressSnapshot) (*FreightTemplateRule, error) {
	if templateID <= 0 {
		return nil, nil
	}

	remote := isRemoteArea(snap.ProvinceCode, snap.CityCode)
	var key string
	if remote {
		key = fmt.Sprintf("shop:freighttemplate:remote:%d:%s:%s", templateID, snap.DistrictCode, snap.StreetCode)
	} else {
		key = fmt.Sprintf("shop:freighttemplate:normal:%d:%s", templateID, snap.DistrictCode)
	}

	// 1. cache hit
	if s.rdb != nil {
		if cached, err := s.rdb.Get(ctx, key).Bytes(); err == nil && len(cached) > 0 {
			var r FreightTemplateRule
			if err := json.Unmarshal(cached, &r); err == nil {
				s.logger.Debug("freight rule cache hit", zap.String("key", key))
				return &r, nil
			}
		}
	}

	// 2. cache miss:由数据库直接选择最优规则
	regionCodes := make([]string, 0, 3)
	for _, code := range []string{snap.DistrictCode, snap.CityCode, snap.ProvinceCode} {
		if code != "" {
			regionCodes = append(regionCodes, code)
		}
	}
	rule, err := s.repo.FindBestRuleForRegion(ctx, templateID, regionCodes)
	if err != nil {
		return nil, fmt.Errorf("find best freight rule: %w", err)
	}
	if rule == nil {
		return nil, nil // 不缓存 nil,后续配置变更后可立即生效
	}

	// 3. 写回 cache
	if s.rdb != nil {
		if data, err := json.Marshal(rule); err == nil {
			if err := s.rdb.Set(ctx, key, data, time.Hour).Err(); err != nil {
				s.logger.Warn("freight rule cache write failed", zap.String("key", key), zap.Error(err))
			}
		}
	}
	return rule, nil
}

// CreateOrder 下单。
//
// CreateOrder 创建订单并计算商品金额、运费和应付金额。
func (s *Service) CreateOrder(ctx context.Context, userID int64, req *CreateOrderReq) (*Order, error) {
	// 收集 SKU 并合并同一 SKU 的数量，确保查询和运费计算都只处理一次。
	skuIDSet := make(map[int64]int, len(req.Items))
	for _, it := range req.Items {
		skuIDSet[int64(it.SkuID)] += it.Qty
	}
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

	if err := s.checkItemsAvailability(req, skuMap); err != nil {
		return nil, err
	}

	// 3.计算运费
	var totalPrice int64
	for _, skuID := range skuIDs {
		info := skuMap[skuID]
		totalPrice += info.SkuPriceCents * int64(skuIDSet[skuID])
	}

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

	freightTemplates, err := s.repo.FindFreightTemplatesBySKUIDs(ctx, skuIDs)
	if err != nil {
		return nil, fmt.Errorf("find freight templates: %w", err)
	}
	templatesByID := make(map[int64]*FreightTemplate, len(freightTemplates))
	for _, template := range freightTemplates {
		if template != nil {
			templatesByID[template.ID] = template
		}
	}

	freight, err := s.calcFreight(ctx, skuMap, skuIDSet, templatesByID, snap, totalPrice)
	if err != nil {
		return nil, err
	}
	total := totalPrice + freight
	return &Order{
		UserID:          userID,
		Status:          "pending",
		GoodsCents:      totalPrice,
		FreightCents:    freight,
		TotalCents:      total,
		PayCents:        total,
		AddressSnapshot: snap,
	}, nil
}
