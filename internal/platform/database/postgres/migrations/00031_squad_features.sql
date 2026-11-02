-- +goose Up
ALTER TABLE squad_product_overrides ADD COLUMN features_json TEXT COLLATE "C";
INSERT INTO schema_migrations(version,applied_at) VALUES('116_squad_features.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
