-- +goose Up
ALTER TABLE combos ADD COLUMN features_json TEXT COLLATE "C" NOT NULL DEFAULT '[]';
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
