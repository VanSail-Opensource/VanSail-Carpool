-- +goose Up
ALTER TABLE pricing_cost_metadata ADD COLUMN charge_mode TEXT COLLATE "C" NOT NULL DEFAULT 'cycle' CHECK(charge_mode IN ('cycle','hourly'));
ALTER TABLE pricing_cost_metadata ADD COLUMN hourly_rate_micros BIGINT NOT NULL DEFAULT 0 CHECK(hourly_rate_micros>=0);
ALTER TABLE pricing_cost_metadata ADD COLUMN traffic_rate_micros BIGINT NOT NULL DEFAULT 0 CHECK(traffic_rate_micros>=0);
ALTER TABLE pricing_cost_metadata ADD COLUMN capacity_unlimited BIGINT NOT NULL DEFAULT 0 CHECK(capacity_unlimited IN (0,1));
ALTER TABLE pricing_cost_versions ADD COLUMN charge_mode TEXT COLLATE "C" NOT NULL DEFAULT 'cycle' CHECK(charge_mode IN ('cycle','hourly'));
ALTER TABLE pricing_cost_versions ADD COLUMN hourly_rate_micros BIGINT NOT NULL DEFAULT 0 CHECK(hourly_rate_micros>=0);
ALTER TABLE pricing_cost_versions ADD COLUMN traffic_rate_micros BIGINT NOT NULL DEFAULT 0 CHECK(traffic_rate_micros>=0);
ALTER TABLE pricing_cost_versions ADD COLUMN capacity_unlimited BIGINT NOT NULL DEFAULT 0 CHECK(capacity_unlimited IN (0,1));

INSERT INTO schema_migrations(version,applied_at) VALUES('111_provider_cost_models.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
