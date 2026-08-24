-- +goose Up
-- +goose StatementBegin

-- 整单免运阈值(全局生效,不绑模板)
-- 一张表最多 2 行:is_remote=false(普通) / true(偏远)各一条
-- 找不到记录 = 该区域不启用整单免运,按 product 正常算
CREATE TABLE freight_order_threshold (
    id              bigserial    PRIMARY KEY,
    -- 满 X 分免运。CHECK > 0 防止 0 值导致"任何 order 都免运"的逻辑错误
    threshold_cents bigint       NOT NULL CHECK (threshold_cents > 0),
    -- 适用区域:true=偏远地区 / false=普通地区
    is_remote       boolean      NOT NULL DEFAULT false,
    remark          varchar(256) NOT NULL DEFAULT '',
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now(),

    -- 每个区域类别只能配一条,避免重复配置
    CONSTRAINT uk_freight_order_threshold_remote UNIQUE (is_remote)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS freight_order_threshold;
-- +goose StatementEnd