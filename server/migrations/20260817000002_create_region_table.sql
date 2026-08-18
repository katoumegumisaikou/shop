-- +goose Up
-- +goose StatementBegin

-- 行政区划表（省/市/区三级）
CREATE TABLE region (
    id         bigserial PRIMARY KEY,
    code       varchar(32) NOT NULL,
    parent_id  bigint NOT NULL DEFAULT 0,
    name       varchar(64) NOT NULL,
    level      int NOT NULL DEFAULT 1,
    sort       int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_region_code UNIQUE (code)
);
CREATE INDEX idx_region_parent_id ON region(parent_id);
CREATE INDEX idx_region_level ON region(level);

-- 自引用外键：parent_id 必须指向 region.id
-- 注意：seed 时必须先插顶级（parent_id=0），再插子级，
-- 否则 FK 约束会拒绝插入（go 端 seedRegionsIntoDB 也是两遍插入保证顺序）。
ALTER TABLE region
    ADD CONSTRAINT fk_region_parent
    FOREIGN KEY (parent_id) REFERENCES region(id)
    ON DELETE RESTRICT
    ON UPDATE CASCADE;

CREATE TRIGGER trg_region_set_updated_at
BEFORE UPDATE ON region
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_region_set_updated_at ON region;
ALTER TABLE region DROP CONSTRAINT IF EXISTS fk_region_parent;
DROP INDEX IF EXISTS idx_region_level;
DROP INDEX IF EXISTS idx_region_parent_id;
DROP TABLE IF EXISTS region;

-- +goose StatementEnd