-- +goose Up
-- Previous readiness evidence did not bind owner generation or credential
-- material and could survive purge/rejoin. Reconfirm each member once from a
-- fresh authenticated configuration. Migration runs before the master starts;
-- offline owners wait for a new applied acknowledgment before revealing links.
UPDATE xnode_identities SET first_synced_at=NULL
 WHERE user_id IS NOT NULL AND first_synced_at IS NOT NULL;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
