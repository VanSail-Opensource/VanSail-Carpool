-- +goose Up
-- A late barrier from a prior closure must not seal a reopened period.
ALTER TABLE xnode_periods ADD COLUMN closure_generation BIGINT NOT NULL DEFAULT 0 CHECK(closure_generation>=0);
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
