-- +goose Up
-- +goose StatementBegin

CREATE TABLE "user" (
  id                 bigserial PRIMARY KEY,
  openid_mp          text UNIQUE,
  openid_h5          text UNIQUE,
  unionid            text UNIQUE,
  phone              text UNIQUE,
  phone_country      text NOT NULL DEFAULT '86',
  password_hash      text,
  source             text NOT NULL DEFAULT 'mp',
  nickname           text,
  avatar             text,
  gender             integer NOT NULL DEFAULT 0,
  birthday           date,
  status             text NOT NULL DEFAULT 'active',
  deactivate_at      timestamptz,
  invited_by_user_id bigint,
  distributor_id     bigint,
  points             integer NOT NULL DEFAULT 0,
  balance_cents      bigint NOT NULL DEFAULT 0,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_invited_by_user_id ON "user"(invited_by_user_id);
CREATE INDEX idx_user_distributor_id ON "user"(distributor_id);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_user_set_updated_at
BEFORE UPDATE ON "user"
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_user_set_updated_at ON "user";
DROP FUNCTION IF EXISTS set_updated_at();
DROP INDEX IF EXISTS idx_user_distributor_id;
DROP INDEX IF EXISTS idx_user_invited_by_user_id;
DROP TABLE IF EXISTS "user";

-- +goose StatementEnd
