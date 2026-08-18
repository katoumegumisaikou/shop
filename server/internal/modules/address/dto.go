package address

import "time"

// createReq 新建地址请求。
type createReq struct {
	ReceiverName string `json:"receiver_name" binding:"required,max=64"`
	Phone        string `json:"phone"         binding:"required,max=20"`
	RegionCode   string `json:"region_code"   binding:"required,max=32"`
	Detail       string `json:"detail"        binding:"required,max=256"`
	IsDefault    bool   `json:"is_default"`
}

// updateReq 更新地址请求。指针字段用于区分"未传"与"显式 false"。
type updateReq struct {
	ID           int64  `json:"id" binding:"required"`
	ReceiverName string `json:"receiver_name" binding:"omitempty,max=64"`
	Phone        string `json:"phone"         binding:"omitempty,max=20"`
	RegionCode   string `json:"region_code"   binding:"omitempty,max=32"`
	Detail       string `json:"detail"        binding:"omitempty,max=256"`
	IsDefault    *bool  `json:"is_default,omitempty"`
}

// defaultReq 设为默认地址请求（路由是 POST /default，body 带 id）。
type defaultReq struct {
	ID int64 `json:"id" binding:"required"`
}

// AddressResp 单条地址响应。
type AddressResp struct {
	ID           int64     `json:"id"`
	ReceiverName string    `json:"receiver_name"`
	Phone        string    `json:"phone"`
	RegionCode   string    `json:"region_code"`
	Detail       string    `json:"detail"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// toResp Address 实体 → AddressResp 响应。
func toResp(a *Address) *AddressResp {
	if a == nil {
		return &AddressResp{}
	}
	return &AddressResp{
		ID:           a.ID,
		ReceiverName: a.ReceiverName,
		Phone:        a.Phone,
		RegionCode:   a.RegionCode,
		Detail:       a.Detail,
		IsDefault:    a.IsDefault,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}