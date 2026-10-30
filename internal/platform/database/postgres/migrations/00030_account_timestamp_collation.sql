-- +goose Up
ALTER TABLE users ALTER COLUMN last_purged_at TYPE TEXT COLLATE "C";

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
