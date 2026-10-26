-- +goose Up
CREATE TABLE user_host_names (
 user_id TEXT COLLATE "C" NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 host_id TEXT COLLATE "C" NOT NULL CHECK(length(host_id) BETWEEN 1 AND 256),
 name TEXT COLLATE "C" NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 PRIMARY KEY(user_id,host_id)
);
INSERT INTO schema_migrations(version,applied_at) VALUES('108_host_name_overrides.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
