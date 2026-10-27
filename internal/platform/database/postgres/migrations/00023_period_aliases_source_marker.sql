-- +goose Up
-- Keep the SQLite migration inventory in step with the equivalent PostgreSQL
-- schema, including databases created before the marker was introduced.
INSERT INTO schema_migrations(version,applied_at)
VALUES('109_xnode_period_aliases.sql',vansail_now())
ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
