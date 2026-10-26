-- +goose Up
CREATE TABLE pricing_personal_decisions (
 version TEXT COLLATE "C" PRIMARY KEY,
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 product_type TEXT COLLATE "C" NOT NULL CHECK (product_type IN ('combo','addon','payg')),
 product_id TEXT COLLATE "C" NOT NULL,
 context_key TEXT COLLATE "C" NOT NULL,
 policy_version BIGINT NOT NULL,
 public_minor BIGINT NOT NULL CHECK(public_minor>=0),
 personal_minor BIGINT NOT NULL CHECK(personal_minor>=0),
 net_minor BIGINT NOT NULL CHECK(net_minor>=0),
 status TEXT COLLATE "C" NOT NULL CHECK(status IN ('ADJUSTED','UNCHANGED')),
 reason TEXT COLLATE "C" NOT NULL,
 model_version TEXT COLLATE "C" NOT NULL,
 probability DOUBLE PRECISION,
 calibration TEXT COLLATE "C" NOT NULL,
 generated_at TEXT COLLATE "C" NOT NULL,
 valid_until TEXT COLLATE "C" NOT NULL,
 coupon_json TEXT COLLATE "C" NOT NULL CHECK(jsonb_typeof(coupon_json::jsonb)='array')
);
CREATE INDEX pricing_personal_owner_idx ON pricing_personal_decisions(user_id,product_type,product_id,generated_at);
ALTER TABLE pricing_offers ADD COLUMN outcome_reason TEXT COLLATE "C" NOT NULL DEFAULT '';
ALTER TABLE pricing_renewal_quotes ADD COLUMN outcome_reason TEXT COLLATE "C" NOT NULL DEFAULT '';

INSERT INTO schema_migrations(version,applied_at) VALUES('107_personal_pricing_decisions.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
