-- +goose Up
ALTER TABLE pricing_cost_metadata DROP CONSTRAINT pricing_cost_metadata_capacity_bytes_check;
ALTER TABLE pricing_cost_metadata ADD CONSTRAINT pricing_cost_metadata_capacity_bytes_check CHECK(capacity_bytes>=0 AND (capacity_bytes>0 OR capacity_unlimited=1));

INSERT INTO schema_migrations(version,applied_at) VALUES('112_unlimited_zero_capacity.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
