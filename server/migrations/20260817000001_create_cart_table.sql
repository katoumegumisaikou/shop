-- +goose Up
-- +goose StatementBegin

-- 购物车条目表
-- id 使用 bigint 但由雪花 ID 写入（非 bigserial），保留扩展性
CREATE TABLE cart_item (
    id                  bigint PRIMARY KEY,
    user_id             bigint NOT NULL REFERENCES "user"(id),
    sku_id              bigint NOT NULL REFERENCES sku(id),
    qty                 int NOT NULL,
    snapshot_price_cents bigint NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_cart_item_user_sku UNIQUE (user_id, sku_id)
);

-- 按用户查询列表
CREATE INDEX idx_cart_item_user_id ON cart_item(user_id);

-- 触发器：自动维护 updated_at
CREATE TRIGGER trg_cart_item_set_updated_at
BEFORE UPDATE ON cart_item
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_cart_item_set_updated_at ON cart_item;
DROP INDEX IF EXISTS idx_cart_item_user_id;
DROP TABLE IF EXISTS cart_item;

-- +goose StatementEnd