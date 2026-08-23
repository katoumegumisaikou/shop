-- +goose Up
-- +goose StatementBegin

-- 运费模板
CREATE TABLE freight_template (
    id         bigserial   PRIMARY KEY,
    name       varchar(64) NOT NULL,
    is_default boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- 同一时间只能有一个默认模板(partial unique index)
CREATE UNIQUE INDEX uk_freight_template_default
    ON freight_template(is_default)
    WHERE is_default = true;

CREATE TRIGGER trg_freight_template_set_updated_at
BEFORE UPDATE ON freight_template
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 运费规则(地区覆盖通过 freight_rule_region 多对多绑定)
CREATE TABLE freight_template_rule (
    id                   bigserial   PRIMARY KEY,
    template_id          bigint      NOT NULL,
    -- 可选:SKU 所属运费分组(NULL 表示不分组,适用于整单)
    freight_group_id     bigint,
    -- 计费方式:weight / amount / fixed
    charge_type          varchar(16) NOT NULL DEFAULT 'weight',
    -- 满额免运(三种模式通用),0 表示不启用
    free_threshold_cents bigint      NOT NULL DEFAULT 0,
    -- 按重量计费(charge_type='weight' 时使用)
    first_weight_g       int,
    first_fee_cents      bigint,
    additional_weight_g  int,
    additional_fee_cents bigint,
    -- 固定运费 / 按金额模式的兜底费用(charge_type='fixed'/'amount' 时使用)
    fixed_fee_cents      bigint,
    -- 同 level 内多条规则匹配时按 DESC 选最大
    priority             int         NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_freight_rule_template_id     ON freight_template_rule(template_id);
CREATE INDEX idx_freight_rule_freight_group  ON freight_template_rule(freight_group_id);
CREATE INDEX idx_freight_rule_priority       ON freight_template_rule(priority);

CREATE TRIGGER trg_freight_template_rule_set_updated_at
BEFORE UPDATE ON freight_template_rule
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- 规则-地区多对多绑定(中间表)
-- 一条规则可绑定多个地区,匹配时按 (level, rule.priority DESC) 选最佳
CREATE TABLE freight_rule_region (
    rule_id     bigint      NOT NULL,
    region_code varchar(32) NOT NULL,
    level       varchar(8)  NOT NULL,
    PRIMARY KEY (rule_id, region_code),
    CONSTRAINT fk_freight_rule_region_rule
        FOREIGN KEY (rule_id) REFERENCES freight_template_rule(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_freight_rule_region_code ON freight_rule_region(region_code);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS freight_rule_region;
DROP TRIGGER IF EXISTS trg_freight_template_rule_set_updated_at ON freight_template_rule;
DROP TRIGGER IF EXISTS trg_freight_template_set_updated_at  ON freight_template;
DROP TABLE IF EXISTS freight_template_rule;
DROP TABLE IF EXISTS freight_template;

-- +goose StatementEnd