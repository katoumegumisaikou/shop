package address

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/region"
	"shop/internal/pkg/snowflake"
	"shop/internal/pkg/wxlogin"
)

// WxSessionKeyTTL 微信 session_key 在 Redis 中的缓存时长。
// 微信官方文档建议与 wx.login 后端自定义登录态 token 寿命对齐，这里取 7 天。
const WxSessionKeyTTL = 7 * 24 * time.Hour

// Service 收货地址服务。
type Service struct {
	repo       AddressRepo
	rdb        *redis.Client
	wx         wxlogin.WxLoginClient
	regionRepo region.Repo
	logger     *zap.Logger
}

// NewService 构造 Service。
//
//	rdb:        用于读取微信 session_key（解密 wx 加密地址用）；为 nil 时 DecryptWxAddress 直接返回 503。
//	wx:         实际解密逻辑（带 appid 校验）交给 wxlogin 客户端；为 nil 时 DecryptWxAddress 返回 503。
//	regionRepo: 行政区划查询；为 nil 时 Region 直接返回 503。
func NewService(repo AddressRepo, rdb *redis.Client, wx wxlogin.WxLoginClient, regionRepo region.Repo, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, rdb: rdb, wx: wx, regionRepo: regionRepo, logger: logger}
}

// Get 查询单条地址（仅本人可见）。
func (s *Service) Get(ctx context.Context, userID, id int64) (*AddressResp, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrInternal
	}
	if a.UserID != userID {
		return nil, errs.ErrForbidden
	}
	return toResp(a), nil
}

// List 列出当前用户全部地址（按默认优先 + id 倒序）。
func (s *Service) List(ctx context.Context, userID int64) ([]*AddressResp, error) {
	list, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]*AddressResp, 0, len(list))
	for _, a := range list {
		out = append(out, toResp(a))
	}
	return out, nil
}

// Create 新建地址。IsDefault=true 时,repo 在事务内先清同用户其他默认。
func (s *Service) Create(ctx context.Context, userID int64, req *createReq) (*AddressResp, error) {
	addr := &Address{
		ID:           snowflake.NextID(),
		UserID:       userID,
		ReceiverName: req.ReceiverName,
		Phone:        req.Phone,
		RegionCode:   req.RegionCode,
		Detail:       req.Detail,
		IsDefault:    req.IsDefault,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.repo.InsertAddress(ctx, addr); err != nil {
		s.logger.Error("insert address failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, errs.ErrInternal
	}
	return toResp(addr), nil
}

// Update 更新指定地址。
//   - 字段为空字符串/缺省时不动（除了 IsDefault 用指针区分"未传"和"显式 false"）
//   - 归属校验:不是本人的地址返回 ErrForbidden
//   - IsDefault=true 时,repo 在事务内先清其他默认
func (s *Service) Update(ctx context.Context, userID int64, req *updateReq) (*AddressResp, error) {
	a, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrInternal
	}
	if a.UserID != userID {
		return nil, errs.ErrForbidden
	}

	if req.ReceiverName != "" {
		a.ReceiverName = req.ReceiverName
	}
	if req.Phone != "" {
		a.Phone = req.Phone
	}
	if req.RegionCode != "" {
		a.RegionCode = req.RegionCode
	}
	if req.Detail != "" {
		a.Detail = req.Detail
	}
	if req.IsDefault != nil {
		a.IsDefault = *req.IsDefault
	}

	if err := s.repo.UpdateAddress(ctx, a); err != nil {
		s.logger.Error("update address failed", zap.Int64("id", a.ID), zap.Error(err))
		return nil, errs.ErrInternal
	}
	return toResp(a), nil
}

// Delete 删除指定地址（仅本人）。
//
//	rows=0 视为"找不到或非本人"，统一返回 ErrNotFound 不暴露区分。
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	rows, err := s.repo.DeleteByUserID(ctx, userID, id)
	if err != nil {
		s.logger.Error("delete address failed", zap.Int64("id", id), zap.Error(err))
		return errs.ErrInternal
	}
	if rows == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// SetDefault 把指定地址设为默认（先清其他,再设当前）。
func (s *Service) SetDefault(ctx context.Context, userID, id int64) error {
	rows, err := s.repo.SetDefault(ctx, userID, id)
	if err != nil {
		return errs.ErrInternal
	}
	if rows == 0 {
		// id 不存在或不属于该用户
		return errs.ErrNotFound
	}
	return nil
}

// DecryptWxAddress 解密微信加密地址。
//
// 流程:
//  1. 从 Redis 取该用户的 wx session_key（wx 登录时由 account 模块写入）
//  2. 委托给 wxlogin.Client.DecryptUserData（带 appid 校验）
//  3. 返回解密后的 map（含 provinceName/cityName/countyName/detail 等字段）
//
// session_key 的 Redis key 约定与 wx 登录模块保持一致：
//
//	当前约定 `shop:wx:session:{userID}`，TTL 7 天，由 SaveWxSessionKey 写入。
func (s *Service) DecryptWxAddress(ctx context.Context, userID int64, encryptedData, iv string) (map[string]any, error) {
	if s.rdb == nil {
		return nil, errs.ErrServiceDegraded.WithMsg("Redis 未配置,无法解密微信地址")
	}
	if s.wx == nil {
		return nil, errs.ErrServiceDegraded.WithMsg("微信客户端未配置,无法解密微信地址")
	}

	sessionKey, err := s.rdb.Get(ctx, WxSessionKey(userID)).Result()
	if err == redis.Nil {
		return nil, errs.ErrSessionExpired
	}
	if err != nil {
		s.logger.Error("get wx session_key failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, errs.ErrInternal
	}

	data, err := s.wx.DecryptUserData(sessionKey, encryptedData, iv)
	if err != nil {
		s.logger.Warn("decrypt wx data failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, errs.ErrParam.WithMsg("微信加密数据解密失败,请重新授权")
	}
	return data, nil
}

// WxSessionKey 拼出 wx session_key 的 Redis key。
// 调用方应在 wx 登录成功后用 SET key session_key EX WxSessionKeyTTL 写入。
func WxSessionKey(userID int64) string {
	return fmt.Sprintf("shop:wx:session:%d", userID)
}

// Region 行政区划查询。
//
//   - regionCode 为空:返回顶级(省),ParentCode=nil
//   - regionCode 非空:返回该 region 的下级(市/区),ParentCode=&regionCode
//
// 每条记录携带 HasChildren,指示该 region 是否还有下级(供前端判断是否要继续展开)。
// 单条 SQL 完成查询,无前置 GetByCode。
func (s *Service) Region(ctx context.Context, regionCode string) ([]RegionResp, error) {
	if s.regionRepo == nil {
		return nil, errs.ErrServiceDegraded.WithMsg("region 模块未启用")
	}

	regions, err := s.regionRepo.ListByParentCode(ctx, regionCode)
	if err != nil {
		s.logger.Error("list region by parent_code failed", zap.String("parent_code", regionCode), zap.Error(err))
		return nil, errs.ErrInternal
	}

	var parentCode *string
	if regionCode != "" {
		code := regionCode
		parentCode = &code
	}

	out := make([]RegionResp, len(regions))
	for i, r := range regions {
		out[i] = RegionResp{
			Code:        r.Code,
			ParentCode:  parentCode,
			Name:        r.Name,
			Level:       r.Level,
			HasChildren: r.HasChildren,
		}
	}
	return out, nil
}
