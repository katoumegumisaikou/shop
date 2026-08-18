package region

import (
	"embed"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

//go:embed assets/regions.json
var regionSeedFS embed.FS

type regionSeedRow struct {
	Code       string `json:"code"`
	ParentCode string `json:"parent_code"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	Sort       int    `json:"sort"`
}

func LoadRegion(db *gorm.DB) (int, error) {
	regions, err := parseRegion()
	if err != nil {
		return 0, err
	}

	if err := seedRegionsIntoDB(db, regions); err != nil {
		return 0, err
	}
	return len(regions), nil
}

func parseRegion() ([]regionSeedRow, error) {
	var regions []regionSeedRow

	data, err := regionSeedFS.ReadFile("assets/regions.json")
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &regions); err != nil {
		return nil, err
	}

	return regions, nil
}

// seedRegionsIntoDB 把行政区数据写入 region 表。
//
// 两遍插入策略：
//  1. 先插 parent_code 为空的行（一级行政区，如省），并把 code → id 缓存起来。
//  2. 再插其余行，通过 parent_code 反查父级 id 后写入 parent_id。
//
// 幂等性：使用 ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code 的小技巧，
// 冲突时返回已有 id，未冲突时返回新插入的 id，保证重复 seed 不报错。
//
// 数据完整性兜底：父级不在 seed 数据中时，子级跳过（不报错），由人工介入补齐。
func seedRegionsIntoDB(db *gorm.DB, rows []regionSeedRow) error {
	if len(rows) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		codeToID := make(map[string]int64, len(rows))

		// 第一遍：插入顶级行政区（parent_code 为空）。
		for _, r := range rows {
			if r.ParentCode != "" {
				continue
			}
			id, err := upsertRegion(tx, r.Code, 0, r.Name, r.Level, r.Sort)
			if err != nil {
				return err
			}
			codeToID[r.Code] = id
		}

		// 第二遍：插入子级，通过父级 code 反查 id 后写入 parent_id。
		for _, r := range rows {
			if r.ParentCode == "" {
				continue
			}
			parentID, ok := codeToID[r.ParentCode]
			if !ok {
				// 父级不在本次 seed 数据中：fail-fast 让上层感知脏数据，
				// 避免悄悄写入孤儿行（孤儿会让 region 表的 FK 约束全部触发）。
				return fmt.Errorf("region seed: parent_code=%q 不存在 (子级 code=%q)", r.ParentCode, r.Code)
			}
			id, err := upsertRegion(tx, r.Code, parentID, r.Name, r.Level, r.Sort)
			if err != nil {
				return err
			}
			codeToID[r.Code] = id
		}

		return nil
	})
}

// upsertRegion 幂等 upsert：已存在则返回现有 id，未存在则插入并返回新 id。
// ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code 是一个常用的小技巧：
// UPDATE 的 SET 子句故意赋原值，纯粹为了让 RETURNING 能拿到 id。
func upsertRegion(tx *gorm.DB, code string, parentID int64, name string, level, sort int) (int64, error) {
	var id int64
	err := tx.Raw(`
		INSERT INTO region (code, parent_id, name, level, sort)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code
		RETURNING id
	`, code, parentID, name, level, sort).Scan(&id).Error
	return id, err
}
