package product

import (
	"time"

	"shop/internal/pkg/types"
)

type ListCategoriesResp struct {
	ID        types.Int64Str       `json:"id"`
	ParentID  types.Int64Str       `json:"parent_id"`
	Name      string               `json:"name"`
	Icon      string               `json:"icon,omitempty"`
	Sort      int                  `json:"sort"`
	Status    string               `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	Children  []ListCategoriesResp `json:"children,omitempty"`
}

type ListCategoriesResult ListCategoriesResp
