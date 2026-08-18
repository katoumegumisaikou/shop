-- +goose Up
-- +goose StatementBegin

-- 收货地址表
CREATE TABLE address (
    id            bigserial PRIMARY KEY,
    user_id       bigint NOT NULL,
    receiver_name varchar(64) NOT NULL,
    phone         varchar(20) NOT NULL,
    region_code   varchar(32) NOT NULL,    -- region.code 的软引用（暂不加 FK,等 region 模块正式接入再加）
    detail        varchar(256) NOT NULL,
    is_default    boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_address_user_id ON address(user_id);

-- 部分唯一索引：保证每个用户最多只有一条 is_default=true 的地址
-- PostgreSQL 支持 partial unique index，DB 层兜底"每用户最多一个默认"
CREATE UNIQUE INDEX uk_address_user_default
    ON address(user_id)
    WHERE is_default = true;

CREATE TRIGGER trg_address_set_updated_at
BEFORE UPDATE ON address
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_address_set_updated_at ON address;
DROP INDEX IF EXISTS uk_address_user_default;
DROP INDEX IF EXISTS idx_address_user_id;
DROP TABLE IF EXISTS address;

-- +goose StatementEnd