package address

import (
	"context"
	"time"

	"gorm.io/gorm"
	"go.uber.org/zap"

	"shop/internal/pkg/errs"
	"shop/internal/pkg/snowflake"
)

// Service 收货地址服务。
type Service struct {
	repo   AddressRepo
	logger *zap.Logger
}

// NewService 构造 Service。
func NewService(repo AddressRepo, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, logger: logger}
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
//  - 字段为空字符串/缺省时不动（除了 IsDefault 用指针区分"未传"和"显式 false"）
//  - 归属校验:不是本人的地址返回 ErrForbidden
//  - IsDefault=true 时,repo 在事务内先清其他默认
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
//  rows=0 视为"找不到或非本人"，统一返回 ErrNotFound 不暴露区分。
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