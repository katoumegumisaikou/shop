# modules/cart

> 对应 PRD：`docs/prd/05-cart.md`
> 对应 arch：`docs/arch/05-cart.md`

## 实施阶段
阶段 3

## 文件清单
| 文件 | 内容 |
| --- | --- |
| `entity.go` | `CartItem` / `CartItemDetail` / `CartSKUInfo` |
| `dto.go` | 列表/添加/更新/批量删除/precheck 等请求响应 DTO |
| `repo.go` | `CartRepo` 接口 + GORM 实现 + `FindSKUWithProduct` JOIN |
| `service.go` | 加入合并、上限校验、precheck（结算前校验：可用性/价格/库存） |
| `handler.go` | HTTP handler |
| `router.go` | 仅 C 端，含内联限流（POST /c/cart 60 次/分钟） |
| `service_test.go` | mock CartRepo / ProductRepo，覆盖 Add/Update/Precheck/List |

## API
- `GET  /c/cart` — 购物车列表（带可用性检查）
- `POST /c/cart` — 添加商品（限流 60/min）
- `PUT  /c/cart/:id` — 修改数量
- `DELETE /c/cart/:id` — 删除单条
- `POST /c/cart/batch-delete` — 批量删除
- `POST /c/cart/clean-invalid` — 清除不可用条目
- `GET  /c/cart/count` — 购物车数量
- `POST /c/cart/precheck` — 下单前检查（价格/库存/上下架）

## 本次迁移差异（vs. xu-shop）
1. **无 stock 包**：可用库存走 DB 兜底（`Stock - LockedStock`），未引入 `internal/pkg/stock`。
2. **`FindSKUWithProduct` 替代 SKURepo**：在 `CartRepo` 上新增了一次性 SQL JOIN 返回 SKU + Product 状态，**未对 `product.ProductRepo` 接口做任何改动**。
3. **DTO 拆分**：请求/响应 DTO 单独放 `dto.go`，与 product/account 模块一致。
4. **限流内联**：`POST /c/cart` 的 60/min 限流写在 `router.go` 内部（使用 `redis_rate` + `middleware.RateLimiter`），未新增中间件。
5. **路由装配签名**：`RegisterRoutes(api, handler, rdb, db, userCfg)` 与 product 一致，jwtCfg 直接复用 `pkgjwt.JwtConfig`。
6. **错误风格**：所有错误返回 `*errs.AppError`，由 `response.Error(c, err)` 统一写出 HTTP 状态码与业务码。

## 数据库
- `cart_item` 表迁移：`migrations/20260817000001_create_cart_table.sql`
- `(user_id, sku_id)` UNIQUE 约束 → 支撑 `Upsert` 的 `ON CONFLICT` 合并
- `idx_cart_item_user_id` 索引 → 加速按用户查列表
- `set_updated_at()` 触发器 → 自动维护 `updated_at`