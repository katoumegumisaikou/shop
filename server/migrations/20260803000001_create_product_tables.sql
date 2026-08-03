-- +goose Up
-- +goose StatementBegin

-- 商品分类（支持多级树形结构）
CREATE TABLE category (
    id         bigserial PRIMARY KEY,
    parent_id  bigint NOT NULL DEFAULT 0,
    name       varchar(64) NOT NULL,
    icon       varchar(512),
    sort       int NOT NULL DEFAULT 0,
    status     varchar(16) NOT NULL DEFAULT 'enabled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX idx_category_parent_id ON category(parent_id);
CREATE INDEX idx_category_deleted_at ON category(deleted_at);

CREATE TRIGGER trg_category_set_updated_at
BEFORE UPDATE ON category
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 商品主表
CREATE TABLE product (
    id                  bigserial PRIMARY KEY,
    category_id         bigint NOT NULL,
    title               varchar(60) NOT NULL,
    subtitle            varchar(120),
    main_image          varchar(512) NOT NULL,
    images              jsonb NOT NULL DEFAULT '[]',
    video_url           varchar(512),
    detail_html         text,
    detail_nodes        jsonb,
    status              varchar(16) NOT NULL DEFAULT 'draft',
    sales               int NOT NULL DEFAULT 0,
    sort                int NOT NULL DEFAULT 0,
    tags                jsonb NOT NULL DEFAULT '[]',
    price_min_cents     bigint NOT NULL DEFAULT 0,
    price_max_cents     bigint NOT NULL DEFAULT 0,
    unit                varchar(16) NOT NULL DEFAULT '件',
    is_virtual          boolean NOT NULL DEFAULT false,
    freight_template_id bigint,
    virtual_sales       int NOT NULL DEFAULT 0,
    on_sale_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE INDEX idx_product_category_id ON product(category_id);
CREATE INDEX idx_product_status ON product(status);
CREATE INDEX idx_product_deleted_at ON product(deleted_at);
CREATE INDEX idx_product_sales ON product(sales);

CREATE TRIGGER trg_product_set_updated_at
BEFORE UPDATE ON product
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 规格名（如：颜色、尺寸）
CREATE TABLE product_spec (
    id         bigserial PRIMARY KEY,
    product_id bigint NOT NULL REFERENCES product(id),
    name       varchar(32) NOT NULL,
    sort       int NOT NULL DEFAULT 0
);
CREATE INDEX idx_product_spec_product_id ON product_spec(product_id);

-- 规格值（如：红色、XL）
CREATE TABLE product_spec_value (
    id      bigserial PRIMARY KEY,
    spec_id bigint NOT NULL REFERENCES product_spec(id),
    value   varchar(32) NOT NULL,
    sort    int NOT NULL DEFAULT 0
);
CREATE INDEX idx_product_spec_value_spec_id ON product_spec_value(spec_id);

-- SKU 库存单元
CREATE TABLE sku (
    id                  bigserial PRIMARY KEY,
    product_id          bigint NOT NULL REFERENCES product(id),
    attrs               jsonb NOT NULL DEFAULT '{}',
    price_cents         bigint NOT NULL,
    original_price_cents bigint,
    stock               int NOT NULL DEFAULT 0,
    locked_stock        int NOT NULL DEFAULT 0,
    weight_g            int NOT NULL DEFAULT 0,
    sku_code            varchar(64),
    barcode             varchar(64),
    image               varchar(512),
    status              varchar(16) NOT NULL DEFAULT 'active',
    low_stock_threshold int NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sku_product_id ON sku(product_id);
CREATE INDEX idx_sku_status ON sku(status);
CREATE INDEX idx_sku_sku_code ON sku(sku_code);

CREATE TRIGGER trg_sku_set_updated_at
BEFORE UPDATE ON sku
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 用户浏览历史（联合主键）
CREATE TABLE user_view_history (
    user_id    bigint NOT NULL REFERENCES "user"(id),
    product_id bigint NOT NULL REFERENCES product(id),
    viewed_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, product_id)
);

-- 用户收藏（联合主键）
CREATE TABLE user_favorite (
    user_id    bigint NOT NULL REFERENCES "user"(id),
    product_id bigint NOT NULL REFERENCES product(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, product_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS user_favorite;
DROP TABLE IF EXISTS user_view_history;
DROP TABLE IF EXISTS sku;
DROP TABLE IF EXISTS product_spec_value;
DROP TABLE IF EXISTS product_spec;
DROP TABLE IF EXISTS product;
DROP TABLE IF EXISTS category;

DROP TRIGGER IF EXISTS trg_sku_set_updated_at ON sku;
DROP TRIGGER IF EXISTS trg_product_set_updated_at ON product;
DROP TRIGGER IF EXISTS trg_category_set_updated_at ON category;

-- +goose StatementEnd
