-- +goose Up
-- Existing node revisions do not prove owner membership. Reconnect/sync supplies
-- authenticated configuration evidence; never backfill from node-wide readiness.
ALTER TABLE xnode_identities ADD COLUMN first_synced_at TEXT COLLATE "C";

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
