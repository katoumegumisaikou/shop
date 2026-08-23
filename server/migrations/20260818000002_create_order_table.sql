-- +goose Up
-- +goose StatementBegin

-- 订单主表
-- id 用 bigint NOT NULL(由应用层 snowflake 生成),不用 bigserial
CREATE TABLE order_main (
    id                       bigint       NOT NULL PRIMARY KEY,
    order_no                 varchar(32)  NOT NULL,
    shop_id                  bigint       NOT NULL DEFAULT 1,
    user_id                  bigint       NOT NULL,
    -- 订单状态机：pending / paid / shipped / completed / cancelled / refunding / refunded
    status                   varchar(16)  NOT NULL,
    goods_cents              bigint       NOT NULL,
    freight_cents            bigint       NOT NULL DEFAULT 0,
    discount_cents           bigint       NOT NULL DEFAULT 0,
    coupon_discount_cents    bigint       NOT NULL DEFAULT 0,
    total_cents              bigint       NOT NULL,
    pay_cents                bigint       NOT NULL,
    balance_pay_cents        bigint       NOT NULL DEFAULT 0,
    -- 下单时固化的地址快照(jsonb),不随 address 表变更
    address_snapshot         jsonb        NOT NULL,
    buyer_remark             varchar(200),
    cancel_request_pending   boolean      NOT NULL DEFAULT false,
    cancel_request_reason    varchar(200),
    cancel_request_at        timestamptz,
    cancel_reason            varchar(200),
    distributor_id           bigint,
    distribution_path        jsonb,
    group_buy_order_id       bigint,
    coupon_id                bigint,
    point_used               bigint       NOT NULL DEFAULT 0,
    point_deduct_cents       bigint       NOT NULL DEFAULT 0,
    from_share_user_id       bigint,
    from_channel_code_id     bigint,
    -- 幂等键：与 middleware/idempotency 联动
    idempotency_key          varchar(64),
    current_prepay_id        varchar(64),
    current_prepay_expire_at timestamptz,
    -- 订单过期时间(创建时 +N 分钟),过期后未支付自动关单
    expire_at                timestamptz  NOT NULL,
    paid_at                  timestamptz,
    shipped_at               timestamptz,
    completed_at             timestamptz,
    cancelled_at             timestamptz,
    created_at               timestamptz  NOT NULL DEFAULT now(),
    updated_at               timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT uk_order_main_order_no UNIQUE (order_no)
);

-- 业务侧常用查询索引
CREATE INDEX idx_order_main_user_id            ON order_main(user_id);
CREATE INDEX idx_order_main_user_status        ON order_main(user_id, status);
CREATE INDEX idx_order_main_user_created       ON order_main(user_id, created_at DESC);
CREATE INDEX idx_order_main_status             ON order_main(status);
CREATE INDEX idx_order_main_expire_at           ON order_main(expire_at) WHERE status = 'pending';
CREATE INDEX idx_order_main_idempotency_key     ON order_main(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TRIGGER trg_order_main_set_updated_at
BEFORE UPDATE ON order_main
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 订单明细表
-- id 用 bigint NOT NULL(由应用层 snowflake 生成)
CREATE TABLE order_item (
    id                   bigint      NOT NULL PRIMARY KEY,
    order_id             bigint      NOT NULL,
    sku_id               bigint      NOT NULL,
    product_id           bigint      NOT NULL,
    product_title        varchar(256),
    product_main_image   varchar(512),
    -- 下单时固化的 SKU 规格(如 {"颜色":"红","尺寸":"XL"})
    sku_attrs            jsonb       NOT NULL DEFAULT '{}',
    qty                  int         NOT NULL,
    -- 下单时固化的 SKU 单价(分),不随 sku.price_cents 后续变化
    snapshot_price_cents bigint      NOT NULL,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_item_order_id ON order_item(order_id);
CREATE INDEX idx_order_item_sku_id   ON order_item(sku_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_order_main_set_updated_at ON order_main;
DROP INDEX IF EXISTS idx_order_main_idempotency_key;
DROP INDEX IF EXISTS idx_order_main_expire_at;
DROP INDEX IF EXISTS idx_order_main_status;
DROP INDEX IF EXISTS idx_order_main_user_created;
DROP INDEX IF EXISTS idx_order_main_user_status;
DROP INDEX IF EXISTS idx_order_main_user_id;
DROP INDEX IF EXISTS idx_order_item_sku_id;
DROP INDEX IF EXISTS idx_order_item_order_id;
DROP TABLE IF EXISTS order_item;
DROP TABLE IF EXISTS order_main;

-- +goose StatementEnd